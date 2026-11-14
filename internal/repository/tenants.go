package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// ErrNotFound is returned for scoped lookups that miss: cross-tenant access
// must look identical to a missing row (issue #6).
var ErrNotFound = errors.New("record not found in tenant scope")

// Tenants owns tenant rows. Reads are either by unique slug (setup path) or
// tenant-scoped via the request context.
type Tenants struct{ Q Querier }

const tenantColumns = `id, name, slug, COALESCE(logo_url, ''), theme, status, created_at, updated_at`

func scanTenant(row *sql.Row) (*model.Tenant, error) {
	t := &model.Tenant{}
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.LogoURL, &t.Theme, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, scanErr(err, "tenant")
	}
	return t, nil
}

// Create inserts a tenant.
func (r *Tenants) Create(ctx context.Context, t *model.Tenant) (int64, error) {
	var id int64
	theme := rawOrDefault(t.Theme)
	err := r.Q.QueryRowContext(ctx, `
		INSERT INTO tenants (name, slug, logo_url, theme, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		t.Name, t.Slug, t.LogoURL, theme, t.Status).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, errors.New("tenant slug already taken")
		}
		return 0, scanErr(err, "insert tenant")
	}
	return id, nil
}

// ByID loads a tenant without scope (used by auth flows before a tenant
// context exists and by scoped wrappers below).
func (r *Tenants) ByID(ctx context.Context, id int64) (*model.Tenant, error) {
	return scanTenant(r.Q.QueryRowContext(ctx,
		`SELECT `+tenantColumns+` FROM tenants WHERE id = $1`, id))
}

// Current loads the tenant bound to the request context.
func (r *Tenants) Current(ctx context.Context) (*model.Tenant, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	return r.ByID(ctx, ac.TenantID)
}

// UpdateBranding patches name, logo and theme for the scoped tenant.
func (r *Tenants) UpdateBranding(ctx context.Context, name, logoURL string, theme json.RawMessage) error {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return err
	}
	var themeArg any
	if theme != nil {
		themeArg = theme
	}
	res, err := r.Q.ExecContext(ctx, `
		UPDATE tenants SET
			name = COALESCE($1, name),
			logo_url = COALESCE($2, logo_url),
			theme = COALESCE($3, theme),
			updated_at = now()
		WHERE id = $4`,
		nullString(name), nullString(logoURL), themeArg, ac.TenantID)
	if err != nil {
		return scanErr(err, "update tenant branding")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func rawOrDefault(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

// RowNotFound converts sql.ErrNoRows from scoped scans into ErrNotFound so
// callers cannot distinguish cross-tenant from missing (they should not).
func RowNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
