package repository

import (
	"context"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// Attachments owns attachment rows, always tenant scoped.
type Attachments struct{ Q Querier }

// Insert records an uploaded file.
func (r *Attachments) Insert(ctx context.Context, a *model.Attachment) (int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0, err
	}
	var timeline any
	if a.TimelineID != nil {
		timeline = *a.TimelineID
	}
	var id int64
	err = r.Q.QueryRowContext(ctx, `
		INSERT INTO attachments
			(tenant_id, ticket_id, timeline_id, filename, size_bytes, content_type, storage_path, sha256, uploader_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		ac.TenantID, a.TicketID, timeline, a.Filename, a.SizeBytes, a.ContentType,
		a.StoragePath, a.SHA256, a.UploaderID).Scan(&id)
	if err != nil {
		return 0, scanErr(err, "insert attachment")
	}
	return id, nil
}

// ByID loads one attachment in scope.
func (r *Attachments) ByID(ctx context.Context, id int64) (*model.Attachment, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	a := &model.Attachment{}
	err = r.Q.QueryRowContext(ctx, `
		SELECT id, tenant_id, ticket_id, timeline_id, filename, size_bytes, content_type, storage_path, sha256, uploader_id, created_at
		FROM attachments WHERE tenant_id = $1 AND id = $2`,
		ac.TenantID, id).Scan(&a.ID, &a.TenantID, &a.TicketID, &a.TimelineID, &a.Filename,
		&a.SizeBytes, &a.ContentType, &a.StoragePath, &a.SHA256, &a.UploaderID, &a.CreatedAt)
	if err != nil {
		return nil, RowNotFound(scanErr(err, "attachment"))
	}
	return a, nil
}

// ListByTicket lists attachments of one ticket of the scoped tenant.
func (r *Attachments) ListByTicket(ctx context.Context, ticketID int64) ([]*model.Attachment, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.Q.QueryContext(ctx, `
		SELECT id, tenant_id, ticket_id, timeline_id, filename, size_bytes, content_type, storage_path, sha256, uploader_id, created_at
		FROM attachments WHERE tenant_id = $1 AND ticket_id = $2 ORDER BY id`,
		ac.TenantID, ticketID)
	if err != nil {
		return nil, scanErr(err, "list attachments")
	}
	defer rows.Close()
	var out []*model.Attachment
	for rows.Next() {
		a := &model.Attachment{}
		if err := rows.Scan(&a.ID, &a.TenantID, &a.TicketID, &a.TimelineID, &a.Filename,
			&a.SizeBytes, &a.ContentType, &a.StoragePath, &a.SHA256, &a.UploaderID, &a.CreatedAt); err != nil {
			return nil, scanErr(err, "scan attachment")
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
