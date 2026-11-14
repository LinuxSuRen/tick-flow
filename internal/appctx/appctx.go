// Package appctx carries the authenticated request context (tenant, user, role)
// between the HTTP middleware and the repository layer. Tenant isolation relies
// on every business query reading the scope from here instead of any
// client-provided parameter.
package appctx

import (
	"context"
	"errors"
)

// ErrMissingAuthContext is returned when a business query runs without an
// authenticated scope. Repository methods must treat it as a programming error
// rather than falling back to an unscoped query.
var ErrMissingAuthContext = errors.New("missing authenticated request context")

// AuthContext is injected by the auth middleware after JWT verification.
// The tenant identity is bound to the token; it is never accepted as an
// explicit request parameter.
type AuthContext struct {
	UserID   int64
	TenantID int64
	Role     string
}

type ctxKey struct{}

// With attaches the authenticated context onto ctx.
func With(ctx context.Context, ac *AuthContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, ac)
}

// From extracts the authenticated context, if present.
func From(ctx context.Context) (*AuthContext, bool) {
	ac, ok := ctx.Value(ctxKey{}).(*AuthContext)
	return ac, ok
}

// MustFrom extracts the authenticated context or reports ErrMissingAuthContext.
// It is the entry point used by the repository tenant scope helper.
func MustFrom(ctx context.Context) (*AuthContext, error) {
	if ac, ok := From(ctx); ok && ac != nil {
		return ac, nil
	}
	return nil, ErrMissingAuthContext
}
