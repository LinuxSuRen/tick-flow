package repository

import (
	"context"
	"testing"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
)

// tenantScope is the single source of the tenant filter for business queries.
// A query without an authenticated scope must fail loudly instead of
// silently returning cross-tenant rows (issue #6 test requirement 3).
func tenantScope(ctx context.Context) (int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0, err
	}
	return ac.TenantID, nil
}

func TestTenantScopeRequiresAuthContext(t *testing.T) {
	if _, err := tenantScope(context.Background()); err == nil {
		t.Fatal("expected error for context without auth, got none")
	}
}

func TestTenantScopeReturnsBoundTenant(t *testing.T) {
	ctx := appctx.With(context.Background(), &appctx.AuthContext{
		UserID:   7,
		TenantID: 42,
		Role:     "agent",
	})
	got, err := tenantScope(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected tenant 42, got %d", got)
	}
}
