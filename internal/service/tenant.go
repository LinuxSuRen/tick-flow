package service

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/LinuxSuRen/tick-flow/internal/repository"
)

// TenantService reads the scoped tenant and updates branding (issue #11).
type TenantService struct {
	DB *sql.DB
}

// TenantView is the current tenant projection incl. branding.
type TenantView struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name"`
	Slug    string          `json:"slug"`
	LogoURL string          `json:"logo_url"`
	Theme   json.RawMessage `json:"theme"`
	Status  string          `json:"status"`
}

// CurrentTenant loads the tenant bound to the request context.
func (s *TenantService) CurrentTenant(ctx context.Context) (*TenantView, error) {
	t, err := (&repository.Tenants{Q: s.DB}).Current(ctx)
	if err != nil {
		return nil, err
	}
	return &TenantView{
		ID: t.ID, Name: t.Name, Slug: t.Slug, LogoURL: t.LogoURL,
		Theme: jsonOrDefault(t.Theme), Status: t.Status,
	}, nil
}

// UpdateBranding patches name, logo url and the theme object.
func (s *TenantService) UpdateBranding(ctx context.Context, name, logoURL string, theme json.RawMessage) (*TenantView, error) {
	if err := (&repository.Tenants{Q: s.DB}).UpdateBranding(ctx, name, logoURL, theme); err != nil {
		return nil, err
	}
	return s.CurrentTenant(ctx)
}
