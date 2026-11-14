package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// Tickets owns ticket rows, always tenant scoped. Requesters additionally
// see only their own tickets: the requesterOnly flag on List enforces it.
type Tickets struct{ Q Querier }

const ticketColumns = `
	id, tenant_id, ticket_type_id, number, title, content, priority, state,
	custom_fields, creator_id, assignee_id, due_at, sla_deadlines, closed_at,
	created_at, updated_at`

func scanTicket(row *sql.Row) (*model.Ticket, error) {
	t := &model.Ticket{}
	err := row.Scan(&t.ID, &t.TenantID, &t.TicketTypeID, &t.Number, &t.Title, &t.Content,
		&t.Priority, &t.State, &t.CustomFields, &t.CreatorID, &t.AssigneeID,
		&t.DueAt, &t.SLADeadlines, &t.ClosedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, RowNotFound(scanErr(err, "ticket"))
	}
	return t, nil
}

// nextNumber consumes the per-tenant sequence with a row lock, so
// concurrent creations can never share a number (issue #2 constraint 5).
// It must run inside the creation transaction.
func (r *Tickets) nextNumber(ctx context.Context, tenantID int64) (string, error) {
	var next int64
	err := r.Q.QueryRowContext(ctx, `
		INSERT INTO ticket_sequences (tenant_id, next_value)
		VALUES ($1, 1)
		ON CONFLICT (tenant_id) DO UPDATE SET next_value = ticket_sequences.next_value + 1
		RETURNING next_value - 1`, tenantID).Scan(&next)
	if err != nil {
		return "", scanErr(err, "next ticket number")
	}
	return fmt.Sprintf("%d", next), nil
}

// Create inserts a ticket within the caller's transaction.
func (r *Tickets) Create(ctx context.Context, t *model.Ticket) (*model.Ticket, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	number, err := r.nextNumber(ctx, ac.TenantID)
	if err != nil {
		return nil, err
	}
	t.TenantID = ac.TenantID
	t.Number = number
	var due any
	if t.DueAt != nil {
		due = t.DueAt
	}
	err = r.Q.QueryRowContext(ctx, `
		INSERT INTO tickets
			(tenant_id, ticket_type_id, number, title, content, priority, state,
			 custom_fields, creator_id, due_at, sla_deadlines)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '{}')
		RETURNING id, created_at, updated_at`,
		t.TenantID, t.TicketTypeID, t.Number, t.Title, t.Content, t.Priority, t.State,
		rawOrDefault(t.CustomFields), t.CreatorID, due).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, scanErr(err, "insert ticket")
	}
	return t, nil
}

// TicketFilter carries the list query parameters.
type TicketFilter struct {
	TypeID   *int64
	State    string
	Assignee *int64
	Priority string
	Keyword  string
	// RequesterOnly narrows the list to tickets the caller created.
	RequesterOnly bool
}

// List pages tickets by filter. Requesters are forced to RequesterOnly by
// the service layer before calling here (defense in depth repeats it).
func (r *Tickets) List(ctx context.Context, f TicketFilter, page, pageSize int) ([]*model.Ticket, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	where := []string{"t.tenant_id = $1"}
	args := []any{ac.TenantID}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.TypeID != nil {
		where = append(where, "t.ticket_type_id = "+arg(*f.TypeID))
	}
	if f.State != "" {
		where = append(where, "t.state = "+arg(f.State))
	}
	if f.Assignee != nil {
		where = append(where, "t.assignee_id = "+arg(*f.Assignee))
	}
	if f.Priority != "" {
		where = append(where, "t.priority = "+arg(f.Priority))
	}
	if f.Keyword != "" {
		kw := "%" + f.Keyword + "%"
		where = append(where, "(t.title ILIKE "+arg(kw)+" OR t.number = "+arg(f.Keyword)+")")
	}
	if f.RequesterOnly {
		where = append(where, "t.creator_id = "+arg(ac.UserID))
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.Q.QueryRowContext(ctx,
		`SELECT count(*) FROM tickets t WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, scanErr(err, "count tickets")
	}

	rows, err := r.Q.QueryContext(ctx, `
		SELECT `+ticketColumns+` FROM tickets t
		WHERE `+cond+`
		ORDER BY t.id DESC
		LIMIT `+arg(pageSize)+` OFFSET `+arg(offset(page, pageSize)), args...)
	if err != nil {
		return nil, 0, scanErr(err, "list tickets")
	}
	defer rows.Close()
	var out []*model.Ticket
	for rows.Next() {
		t := &model.Ticket{}
		if err := rows.Scan(&t.ID, &t.TenantID, &t.TicketTypeID, &t.Number, &t.Title, &t.Content,
			&t.Priority, &t.State, &t.CustomFields, &t.CreatorID, &t.AssigneeID,
			&t.DueAt, &t.SLADeadlines, &t.ClosedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, scanErr(err, "scan ticket")
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// ByID loads one ticket in scope.
func (r *Tickets) ByID(ctx context.Context, id int64) (*model.Ticket, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	return scanTicket(r.Q.QueryRowContext(ctx,
		`SELECT `+ticketColumns+` FROM tickets WHERE tenant_id = $1 AND id = $2`,
		ac.TenantID, id))
}

// UpdateState applies a state change, maintaining closed_at (issue #13).
func (r *Tickets) UpdateState(ctx context.Context, id int64, state string, isFinal bool) (*model.Ticket, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	var closedAt any
	if isFinal {
		now := time.Now().UTC()
		closedAt = now
	}
	row := r.Q.QueryRowContext(ctx, `
		UPDATE tickets SET state = $1, closed_at = $2, updated_at = now()
		WHERE tenant_id = $3 AND id = $4
		RETURNING `+ticketColumns,
		state, closedAt, ac.TenantID, id)
	return scanTicket(row)
}

// Assign sets the assignee, writing through to updated_at.
func (r *Tickets) Assign(ctx context.Context, id int64, assigneeID int64) (*model.Ticket, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	row := r.Q.QueryRowContext(ctx, `
		UPDATE tickets SET assignee_id = $1, updated_at = now()
		WHERE tenant_id = $2 AND id = $3
		RETURNING `+ticketColumns,
		assigneeID, ac.TenantID, id)
	return scanTicket(row)
}
