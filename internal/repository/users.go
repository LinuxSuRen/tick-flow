package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// Users owns the global account table. It is deliberately tenant-scope
// free: authentication happens before a tenant context exists.
type Users struct{ Q Querier }

// ErrDuplicateEmail reports a unique violation on users.email.
var ErrDuplicateEmail = errors.New("email already registered")

// ErrValidation reports a request-level validation failure with a
// developer-facing message.
type ErrValidation string

// Error implements error.
func (e ErrValidation) Error() string { return string(e) }

const userColumns = `id, email, password_hash, display_name, status, created_at, updated_at`

func scanUser(row *sql.Row) (*model.User, error) {
	u := &model.User{}
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, scanErr(err, "user")
	}
	return u, nil
}

// Create inserts an account. status follows model (active or invited).
func (r *Users) Create(ctx context.Context, u *model.User) (int64, error) {
	var id int64
	err := r.Q.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, display_name, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		u.Email, u.PasswordHash, u.DisplayName, u.Status).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicateEmail
		}
		return 0, scanErr(err, "insert user")
	}
	return id, nil
}

// ByEmail loads an account by email.
func (r *Users) ByEmail(ctx context.Context, email string) (*model.User, error) {
	return scanUser(r.Q.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

// ByID loads an account by id.
func (r *Users) ByID(ctx context.Context, id int64) (*model.User, error) {
	return scanUser(r.Q.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// SetPassword activates an invited account (or changes a password) and
// flips the status to active.
func (r *Users) SetPassword(ctx context.Context, id int64, hash string) error {
	_, err := r.Q.ExecContext(ctx, `
		UPDATE users SET password_hash = $1, status = 'active', updated_at = now()
		WHERE id = $2`, hash, id)
	if err != nil {
		return scanErr(err, "set password")
	}
	return nil
}

// SystemHasUsers reports whether any account exists (setup guard, issue #6).
func (r *Users) SystemHasUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := r.Q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists)
	if err != nil {
		return false, scanErr(err, "system has users")
	}
	return exists, nil
}

// Count counts accounts (used by tests).
func (r *Users) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.Q.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, scanErr(err, "count users")
}

// isUniqueViolation detects PostgreSQL unique violations across the pgx
// stdlib error types.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
