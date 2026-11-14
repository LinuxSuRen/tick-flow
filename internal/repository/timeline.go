package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// Timeline owns the single-source-of-truth timeline table (issue #2).
type Timeline struct{ Q Querier }

// Insert appends a timeline entry for a ticket of the scoped tenant.
func (r *Timeline) Insert(ctx context.Context, ticketID int64, kind string, authorID int64, body string, meta []byte) (int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0, err
	}
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	var id int64
	err = r.Q.QueryRowContext(ctx, `
		INSERT INTO ticket_timeline (tenant_id, ticket_id, kind, author_id, body, meta)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		ac.TenantID, ticketID, kind, authorID, body, meta).Scan(&id)
	if err != nil {
		return 0, scanErr(err, "insert timeline")
	}
	return id, nil
}

// EntryView is the API projection of a timeline row.
type EntryView struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	AuthorID  int64           `json:"author_id"`
	Author    string          `json:"author"`
	Body      string          `json:"body"`
	Meta      json.RawMessage `json:"meta"`
	CreatedAt string          `json:"created_at"`
}

// ListByTicket pages the timeline of a ticket ascending. Requesters get
// internal notes filtered out (issue #14); the ticket scoping is enforced
// by the ticket lookup that precedes this call.
func (r *Timeline) ListByTicket(ctx context.Context, ticketID int64, includeInternal bool, page, pageSize int) ([]EntryView, int64, error) {
	where := []string{"t.tenant_id = $1", "t.ticket_id = $2"}
	args := []any{mustTenantID(ctx), ticketID}
	if !includeInternal {
		args = append(args, model.KindInternalNote)
		where = append(where, fmt.Sprintf("t.kind != $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.Q.QueryRowContext(ctx,
		`SELECT count(*) FROM ticket_timeline t WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, scanErr(err, "count timeline")
	}

	args = append(args, pageSize, offset(page, pageSize))
	rows, err := r.Q.QueryContext(ctx, `
		SELECT t.id, t.kind, t.author_id, COALESCE(u.display_name, ''), t.body, t.meta, t.created_at
		FROM ticket_timeline t
		LEFT JOIN users u ON u.id = t.author_id
		WHERE `+cond+`
		ORDER BY t.created_at ASC, t.id ASC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, scanErr(err, "list timeline")
	}
	defer rows.Close()
	var out []EntryView
	for rows.Next() {
		var v EntryView
		if err := rows.Scan(&v.ID, &v.Kind, &v.AuthorID, &v.Author, &v.Body, &v.Meta, &v.CreatedAt); err != nil {
			return nil, 0, scanErr(err, "scan timeline")
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func mustTenantID(ctx context.Context) int64 {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0
	}
	return ac.TenantID
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
