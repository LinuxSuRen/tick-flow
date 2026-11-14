package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// TicketTypes owns the template pack rows, always tenant scoped.
type TicketTypes struct{ Q Querier }

const ticketTypeColumns = `
	id, tenant_id, slug, name, COALESCE(icon, ''),
	form_schema, workflow, sla_policy, assignment, view_config,
	is_builtin, status, created_at, updated_at`

func scanTicketType(row *sql.Row) (*model.TicketType, error) {
	t := &model.TicketType{}
	err := row.Scan(&t.ID, &t.TenantID, &t.Slug, &t.Name, &t.Icon,
		&t.FormSchema, &t.Workflow, &t.SLAPolicy, &t.Assignment, &t.ViewConfig,
		&t.IsBuiltin, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, RowNotFound(scanErr(err, "ticket type"))
	}
	return t, nil
}

// Create inserts a ticket type for the scoped tenant.
func (r *TicketTypes) Create(ctx context.Context, t *model.TicketType) (int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.Q.QueryRowContext(ctx, `
		INSERT INTO ticket_types
			(tenant_id, slug, name, icon, form_schema, workflow, sla_policy, assignment, view_config, is_builtin, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		ac.TenantID, t.Slug, t.Name, nullString(t.Icon),
		rawOrDefault(t.FormSchema), rawOrDefault(t.Workflow), rawOrDefault(t.SLAPolicy),
		rawOrDefault(t.Assignment), rawOrDefault(t.ViewConfig),
		t.IsBuiltin, statusOrDefault(t.Status)).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrSlugTaken
		}
		return 0, scanErr(err, "insert ticket type")
	}
	return id, nil
}

// ErrSlugTaken reports a duplicate (tenant, slug).
var ErrSlugTaken = errSlugTaken{}

type errSlugTaken struct{}

func (errSlugTaken) Error() string { return "slug already used in this tenant" }

// ByID loads one ticket type in scope.
func (r *TicketTypes) ByID(ctx context.Context, id int64) (*model.TicketType, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	return scanTicketType(r.Q.QueryRowContext(ctx,
		`SELECT `+ticketTypeColumns+` FROM ticket_types WHERE tenant_id = $1 AND id = $2`,
		ac.TenantID, id))
}

// ListEnabled lists types visible to the portal (enabled only) without
// pagination: the seed set is a handful of rows.
func (r *TicketTypes) ListEnabled(ctx context.Context) ([]*model.TicketType, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.Q.QueryContext(ctx,
		`SELECT `+ticketTypeColumns+` FROM ticket_types
		 WHERE tenant_id = $1 AND status = 'enabled'
		 ORDER BY id`, ac.TenantID)
	if err != nil {
		return nil, scanErr(err, "list ticket types")
	}
	return collectTypes(rows)
}

// ListAll pages every type of the scoped tenant (admin view).
func (r *TicketTypes) ListAll(ctx context.Context, page, pageSize int) ([]*model.TicketType, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.Q.QueryRowContext(ctx,
		`SELECT count(*) FROM ticket_types WHERE tenant_id = $1`, ac.TenantID).Scan(&total); err != nil {
		return nil, 0, scanErr(err, "count ticket types")
	}
	rows, err := r.Q.QueryContext(ctx,
		`SELECT `+ticketTypeColumns+` FROM ticket_types
		 WHERE tenant_id = $1 ORDER BY id LIMIT $2 OFFSET $3`,
		ac.TenantID, pageSize, offset(page, pageSize))
	if err != nil {
		return nil, 0, scanErr(err, "list all ticket types")
	}
	items, err := collectTypes(rows)
	return items, total, err
}

func collectTypes(rows *sql.Rows) ([]*model.TicketType, error) {
	defer rows.Close()
	var out []*model.TicketType
	for rows.Next() {
		t := &model.TicketType{}
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Slug, &t.Name, &t.Icon,
			&t.FormSchema, &t.Workflow, &t.SLAPolicy, &t.Assignment, &t.ViewConfig,
			&t.IsBuiltin, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, scanErr(err, "scan ticket type")
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Update rewrites the mutable columns of one type in scope.
func (r *TicketTypes) Update(ctx context.Context, id int64, t *model.TicketType) error {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return err
	}
	res, err := r.Q.ExecContext(ctx, `
		UPDATE ticket_types SET
			slug = $1, name = $2, icon = $3,
			form_schema = $4, workflow = $5, sla_policy = $6,
			assignment = $7, view_config = $8,
			updated_at = now()
		WHERE tenant_id = $9 AND id = $10`,
		t.Slug, t.Name, nullString(t.Icon),
		rawOrDefault(t.FormSchema), rawOrDefault(t.Workflow), rawOrDefault(t.SLAPolicy),
		rawOrDefault(t.Assignment), rawOrDefault(t.ViewConfig),
		ac.TenantID, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrSlugTaken
		}
		return scanErr(err, "update ticket type")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus enables or disables one type in scope.
func (r *TicketTypes) SetStatus(ctx context.Context, id int64, status string) error {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return err
	}
	res, err := r.Q.ExecContext(ctx, `
		UPDATE ticket_types SET status = $1, updated_at = now()
		WHERE tenant_id = $2 AND id = $3`, status, ac.TenantID, id)
	if err != nil {
		return scanErr(err, "set ticket type status")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateForTenant inserts a seed type for an explicit tenant (import path,
// no request scope).
func (r *TicketTypes) CreateForTenant(ctx context.Context, tenantID int64, t *model.TicketType) error {
	_, err := r.Q.ExecContext(ctx, `
		INSERT INTO ticket_types
			(tenant_id, slug, name, icon, form_schema, workflow, sla_policy, assignment, view_config, is_builtin, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tenant_id, slug) DO NOTHING`,
		tenantID, t.Slug, t.Name, nullString(t.Icon),
		rawOrDefault(t.FormSchema), rawOrDefault(t.Workflow), rawOrDefault(t.SLAPolicy),
		rawOrDefault(t.Assignment), rawOrDefault(t.ViewConfig),
		t.IsBuiltin, statusOrDefault(t.Status))
	if err != nil {
		return scanErr(err, "import seed ticket type")
	}
	return nil
}

func statusOrDefault(status string) string {
	if status == "" {
		return model.StatusEnabled
	}
	return status
}

// jsonbOrNull keeps raw JSON columns symmetric on write.
func jsonbOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
