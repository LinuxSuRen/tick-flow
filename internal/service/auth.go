package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
)

// Sentinel service errors mapped to HTTP codes by the handlers.
var (
	ErrAlreadySetup      = errors.New("system is already initialized")
	ErrBadCredentials    = errors.New("email or password is incorrect")
	ErrAccountNotActive  = errors.New("account has not set a password yet")
	ErrNotAMember        = errors.New("not a member of this tenant")
	ErrWeakPassword      = errors.New("password must be at least 8 characters")
	ErrInvalidTenantSlug = errors.New("tenant slug must be lowercase letters, digits or dashes")
)

// tenantSlugRe allows 2+ chars: lowercase letters, digits and dashes,
// starting with a letter.
var tenantSlugRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,38}[a-z0-9]$`)

// Seeder imports the builtin template packs for a new tenant (issue #19).
type Seeder interface {
	Import(ctx context.Context, q repository.Querier, tenantID int64) error
}

// AuthService implements setup, login, tenant selection and the invited
// account activation (issue #11).
type AuthService struct {
	DB        *sql.DB
	JWTSecret string
	Seeder    Seeder
	Mailer    *Mailer
}

// LoginResult is the shared response of setup, login and select-tenant.
type LoginResult struct {
	Token string      `json:"token"`
	Me   MeView      `json:"me"`
}

// MeView is the /api/me projection.
type MeView struct {
	User    *UserView              `json:"user"`
	Tenant  *CurrentTenantView     `json:"tenant"`
	Tenants []repository.TenantBrief `json:"tenants"`
}

// UserView is the safe user projection.
type UserView struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// CurrentTenantView describes the tenant bound to the token.
type CurrentTenantView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role"`
}

// SetupRequest is the first-boot payload.
type SetupRequest struct {
	TenantName    string `json:"tenant_name"`
	Slug          string `json:"slug"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
	AdminName     string `json:"admin_name"`
}

// Setup creates the first tenant, its admin and the seed types. It fails
// once any user exists (issue #6: setup is one-shot).
func (s *AuthService) Setup(ctx context.Context, req SetupRequest) (*LoginResult, error) {
	users := &repository.Users{Q: s.DB}
	has, err := users.SystemHasUsers(ctx)
	if err != nil {
		return nil, err
	}
	if has {
		return nil, ErrAlreadySetup
	}
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	req.AdminEmail = strings.ToLower(strings.TrimSpace(req.AdminEmail))
	if req.TenantName == "" || req.AdminName == "" || req.AdminEmail == "" {
		return nil, repository.ErrValidation("tenant_name, admin_name and admin_email are required")
	}
	if !tenantSlugRe.MatchString(req.Slug) {
		return nil, ErrInvalidTenantSlug
	}
	if len(req.AdminPassword) < 8 {
		return nil, ErrWeakPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var result *LoginResult
	err = repository.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		tenants := &repository.Tenants{Q: tx}
		tenantID, err := tenants.Create(ctx, &model.Tenant{
			Name:   req.TenantName,
			Slug:   req.Slug,
			Status: model.StatusActive,
		})
		if err != nil {
			return err
		}
		userID, err := users.Create(ctx, &model.User{
			Email:        req.AdminEmail,
			PasswordHash: string(hash),
			DisplayName:  req.AdminName,
			Status:       model.UserStatusActive,
		})
		if err != nil {
			return err
		}
		members := &repository.Members{Q: tx}
		if err := members.Create(ctx, tenantID, userID, model.RoleTenantAdmin); err != nil {
			return err
		}
		if s.Seeder != nil {
			if err := s.Seeder.Import(ctx, tx, tenantID); err != nil {
				return err
			}
		}
		tenant, err := tenants.ByID(ctx, tenantID)
		if err != nil {
			return err
		}
		token, err := middleware.IssueToken(s.JWTSecret, userID, tenantID, model.RoleTenantAdmin, time.Now())
		if err != nil {
			return err
		}
		result = &LoginResult{
			Token: token,
			Me: MeView{
				User: &UserView{ID: userID, Email: req.AdminEmail, DisplayName: req.AdminName},
				Tenant: &CurrentTenantView{
					ID: tenant.ID, Name: tenant.Name, Slug: tenant.Slug, Role: model.RoleTenantAdmin,
				},
				Tenants: []repository.TenantBrief{{
					ID: tenant.ID, Name: tenant.Name, Slug: tenant.Slug, Role: model.RoleTenantAdmin,
				}},
			},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slog.Info("first tenant set up", "tenant_id", result.Me.Tenant.ID, "admin", req.AdminEmail)
	return result, nil
}

// Login verifies credentials and returns a token plus the joinable
// tenants. The token is intentionally tenant-free; select-tenant binds it.
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := (&repository.Users{Q: s.DB}).ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBadCredentials
		}
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, ErrAccountNotActive
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrBadCredentials
	}
	tenants, err := (&repository.Members{Q: s.DB}).TenantsOfUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	token, err := middleware.IssueToken(s.JWTSecret, user.ID, 0, "", time.Now())
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token: token,
		Me: MeView{
			User:    &UserView{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName},
			Tenants: tenants,
		},
	}, nil
}

// SelectTenant re-issues the token bound to a tenant the user joined.
func (s *AuthService) SelectTenant(ctx context.Context, userID, tenantID int64) (*LoginResult, error) {
	user, err := (&repository.Users{Q: s.DB}).ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	members := &repository.Members{Q: s.DB}
	role, err := members.Membership(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotAMember
		}
		return nil, err
	}
	tenant, err := (&repository.Tenants{Q: s.DB}).ByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	tenants, err := members.TenantsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	token, err := middleware.IssueToken(s.JWTSecret, userID, tenantID, role, time.Now())
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token: token,
		Me: MeView{
			User: &UserView{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName},
			Tenant: &CurrentTenantView{
				ID: tenant.ID, Name: tenant.Name, Slug: tenant.Slug, Role: role,
			},
			Tenants: tenants,
		},
	}, nil
}

// Me builds the current user view; unread notification count rides along
// (issue #16 decision).
func (s *AuthService) Me(ctx context.Context) (*MeView, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	user, err := (&repository.Users{Q: s.DB}).ByID(ctx, ac.UserID)
	if err != nil {
		return nil, 0, err
	}
	view := MeView{
		User: &UserView{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName},
	}
	var unread int64
	if ac.TenantID > 0 {
		tenant, err := (&repository.Tenants{Q: s.DB}).ByID(ctx, ac.TenantID)
		if err != nil {
			return nil, 0, err
		}
		view.Tenant = &CurrentTenantView{
			ID: tenant.ID, Name: tenant.Name, Slug: tenant.Slug, Role: ac.Role,
		}
		tenants, err := (&repository.Members{Q: s.DB}).TenantsOfUser(ctx, ac.UserID)
		if err != nil {
			return nil, 0, err
		}
		view.Tenants = tenants
		unread, err = (&repository.Notifications{Q: s.DB}).CountUnread(ctx, ac.UserID)
		if err != nil {
			return nil, 0, err
		}
	}
	return &view, unread, nil
}

// AcceptInvitation activates an invited account with a fresh password
// (single-use semantics: the token only matches invited accounts).
func (s *AuthService) AcceptInvitation(ctx context.Context, token, password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	invite, err := parseInvite(s.JWTSecret, token)
	if err != nil {
		return err
	}
	users := &repository.Users{Q: s.DB}
	user, err := users.ByEmail(ctx, invite.Email)
	if err != nil {
		return ErrInvalidInvite
	}
	if user.Status != model.UserStatusInvited {
		return ErrInvalidInvite
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := users.SetPassword(ctx, user.ID, string(hash)); err != nil {
		return err
	}
	slog.Info("invited account activated", "email", invite.Email, "tenant_id", invite.TenantID)
	return nil
}
