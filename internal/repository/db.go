// Package repository owns all data access. Handlers never write SQL; every
// query goes through a repository method that applies the tenant scope from
// the authenticated request context (issue #6).
package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
)

// ErrNoTenantScope reports a business query attempted without an
// authenticated tenant context. Exposing it to the client would be wrong:
// it indicates a server-side wiring mistake.
var ErrNoTenantScope = appctx.ErrMissingAuthContext

// Open establishes the PostgreSQL connection pool via the pgx stdlib driver.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	return db, nil
}

// Migrate applies every pending up migration embedded in migrationsFS.
// Missing migrations (clean database) is the normal first-boot path.
func Migrate(db *sql.DB, migrationsFS fs.FS) error {
	source, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("init migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx", driver)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
