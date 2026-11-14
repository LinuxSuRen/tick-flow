package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// Querier abstracts *sql.DB and *sql.Tx so repositories can run standalone
// or inside a service-managed transaction.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// WithTx runs fn inside a transaction, committing on success and rolling
// back on error or panic.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

// errNoRowsReporter is a tiny helper for scan sites that need the raw
// sql.ErrNoRows sentinel to bubble up unchanged.
func scanErr(err error, what string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("scan %s: %w", what, err)
}
