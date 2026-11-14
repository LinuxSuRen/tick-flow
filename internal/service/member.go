package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
)

// InviteResult carries the created member view plus the invitation token
// so the admin can hand the set-password link to the invitee (email or
// any out-of-band channel; the server log mirrors the link, issue #6).
type InviteResult struct {
	Member      *repository.MemberView `json:"member"`
	InviteToken string                 `json:"invite_token"`
}

// MemberService implements the member management flows (issue #11).
type MemberService struct {
	DB        *sql.DB
	JWTSecret string
	Mailer    *Mailer
}

var validRoles = map[string]struct{}{
	model.RoleTenantAdmin: {},
	model.RoleAgent:       {},
	model.RoleRequester:   {},
}

// Invite adds an account to the scoped tenant. New accounts start invited
// (no password) and receive a one-time set-password link by mail, or via
// the server log when SMTP is absent (issue #6).
func (s *MemberService) Invite(ctx context.Context, email, role, displayName string) (*InviteResult, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := validRoles[role]; !ok {
		return nil, repository.ErrValidation("role must be tenant_admin, agent or requester")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, repository.ErrValidation("a valid email is required")
	}

	var result *InviteResult
	var inviteTokenStr string
	err = repository.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		users := &repository.Users{Q: tx}
		user, err := users.ByEmail(ctx, email)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			placeholder, err := randomPassword()
			if err != nil {
				return err
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(placeholder), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if displayName == "" {
				displayName = email[:strings.Index(email, "@")]
			}
			userID, err := users.Create(ctx, &model.User{
				Email:        email,
				PasswordHash: string(hash),
				DisplayName:  displayName,
				Status:       model.UserStatusInvited,
			})
			if err != nil {
				return err
			}
			user = &model.User{ID: userID, Email: email, DisplayName: displayName, Status: model.UserStatusInvited}
		}
		members := &repository.Members{Q: tx}
		if err := members.Create(ctx, ac.TenantID, user.ID, role); err != nil {
			return err
		}
		token, err := signInvite(s.JWTSecret, inviteToken{
			TenantID: ac.TenantID,
			Email:    email,
			Role:     role,
			Expires:  time.Now().Add(inviteTokenLifetime),
		})
		if err != nil {
			return err
		}
		inviteTokenStr = token
		result = &InviteResult{
			Member: &repository.MemberView{
				UserID:      user.ID,
				Email:       user.Email,
				DisplayName: user.DisplayName,
				Role:        role,
				Status:      user.Status,
			},
			InviteToken: token,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Notification delivery never breaks the invite flow.
	inviteLink := "/accept-invitation?token=" + inviteTokenStr
	go func() {
		subject := "你被邀请加入 TickFlow 工作区"
		body := fmt.Sprintf("请通过以下链接设置密码并激活账号：\n\n%s\n\n链接 72 小时内有效，仅可使用一次。", inviteLink)
		if err := s.Mailer.Send(email, subject, body); err != nil {
			slog.Error("send invitation mail failed", "email", email, "error", err)
		}
	}()
	slog.Info("member invited", "tenant_id", ac.TenantID, "email", email, "role", role,
		"invite_link", inviteLink)
	return result, nil
}

// List pages the members of the scoped tenant.
func (s *MemberService) List(ctx context.Context, page, pageSize int) ([]repository.MemberView, int64, error) {
	return (&repository.Members{Q: s.DB}).List(ctx, page, pageSize)
}

// UpdateRole changes a member role within the scoped tenant.
func (s *MemberService) UpdateRole(ctx context.Context, memberID int64, role string) (*repository.MemberView, error) {
	if _, ok := validRoles[role]; !ok {
		return nil, repository.ErrValidation("role must be tenant_admin, agent or requester")
	}
	members := &repository.Members{Q: s.DB}
	m, err := members.ByID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if err := members.UpdateRole(ctx, memberID, role); err != nil {
		return nil, err
	}
	return s.viewOf(ctx, m.UserID, role)
}

// Remove drops a membership. The global account stays (it may belong to
// other tenants; tickets keep their creator reference).
func (s *MemberService) Remove(ctx context.Context, memberID int64) error {
	return (&repository.Members{Q: s.DB}).Delete(ctx, memberID)
}

func (s *MemberService) viewOf(ctx context.Context, userID int64, role string) (*repository.MemberView, error) {
	user, err := (&repository.Users{Q: s.DB}).ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &repository.MemberView{
		UserID:      user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        role,
		Status:      user.Status,
	}, nil
}
