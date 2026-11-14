package repository

import (
	"context"
	"database/sql"
)

// Notifications owns in-app notification rows, scoped by user id (the
// tenant_id column stays for audit; the reader is always the user).
type Notifications struct{ Q Querier }

// NotificationView is the API projection.
type NotificationView struct {
	ID        int64   `json:"id"`
	Kind      string  `json:"kind"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	TicketID  *int64  `json:"ticket_id"`
	ReadAt    *string `json:"read_at"`
	CreatedAt string  `json:"created_at"`
}

// InsertBatch writes one notification per recipient.
func (r *Notifications) InsertBatch(ctx context.Context, tenantID int64, userIDs []int64, kind, title, body string, ticketID *int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	var ticket any
	if ticketID != nil {
		ticket = *ticketID
	}
	for _, uid := range userIDs {
		if _, err := r.Q.ExecContext(ctx, `
			INSERT INTO notifications (tenant_id, user_id, ticket_id, kind, title, body)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			tenantID, uid, ticket, kind, title, body); err != nil {
			return scanErr(err, "insert notification")
		}
	}
	return nil
}

// ListByUser pages the notifications of the authenticated user.
func (r *Notifications) ListByUser(ctx context.Context, userID int64, unreadOnly bool, page, pageSize int) ([]NotificationView, int64, error) {
	where := "user_id = $1"
	args := []any{userID}
	if unreadOnly {
		where += " AND read_at IS NULL"
	}
	var total int64
	if err := r.Q.QueryRowContext(ctx,
		`SELECT count(*) FROM notifications WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, scanErr(err, "count notifications")
	}
	args = append(args, pageSize, offset(page, pageSize))
	rows, err := r.Q.QueryContext(ctx, `
		SELECT id, kind, title, body, ticket_id, read_at, created_at
		FROM notifications
		WHERE `+where+`
		ORDER BY id DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, scanErr(err, "list notifications")
	}
	defer rows.Close()
	var out []NotificationView
	for rows.Next() {
		var v NotificationView
		if err := rows.Scan(&v.ID, &v.Kind, &v.Title, &v.Body, &v.TicketID, &v.ReadAt, &v.CreatedAt); err != nil {
			return nil, 0, scanErr(err, "scan notification")
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// MarkRead marks the given ids (or everything when ids is empty) read for
// the authenticated user and reports how many rows changed.
func (r *Notifications) MarkRead(ctx context.Context, userID int64, ids []int64) (int64, error) {
	var res sql.Result
	var err error
	if len(ids) == 0 {
		res, err = r.Q.ExecContext(ctx, `
			UPDATE notifications SET read_at = now()
			WHERE user_id = $1 AND read_at IS NULL`, userID)
	} else {
		res, err = r.Q.ExecContext(ctx, `
			UPDATE notifications SET read_at = now()
			WHERE user_id = $1 AND read_at IS NULL AND id = ANY($2)`, userID, ids)
	}
	if err != nil {
		return 0, scanErr(err, "mark notifications read")
	}
	return res.RowsAffected()
}

// CountUnread returns the unread count for the authenticated user.
func (r *Notifications) CountUnread(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := r.Q.QueryRowContext(ctx, `
		SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`,
		userID).Scan(&n)
	return n, scanErr(err, "count unread notifications")
}
