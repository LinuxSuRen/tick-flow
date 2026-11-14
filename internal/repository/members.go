package repository

import (
	"context"
	"errors"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/model"
)

// ErrAlreadyMember reports a duplicate tenant membership.
var ErrAlreadyMember = errors.New("user is already a member of this tenant")

// MemberView is the list projection of a tenant member.
type MemberView struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// Members owns tenant membership rows.
type Members struct{ Q Querier }

// Create links a user to the scoped tenant with a role.
func (r *Members) Create(ctx context.Context, tenantID, userID int64, role string) error {
	_, err := r.Q.ExecContext(ctx, `
		INSERT INTO tenant_members (tenant_id, user_id, role)
		VALUES ($1, $2, $3)`, tenantID, userID, role)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyMember
		}
		return scanErr(err, "insert member")
	}
	return nil
}

// List pages members of the scoped tenant.
func (r *Members) List(ctx context.Context, page, pageSize int) ([]MemberView, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.Q.QueryRowContext(ctx,
		`SELECT count(*) FROM tenant_members WHERE tenant_id = $1`, ac.TenantID).Scan(&total); err != nil {
		return nil, 0, scanErr(err, "count members")
	}
	rows, err := r.Q.QueryContext(ctx, `
		SELECT m.id, m.user_id, u.email, u.display_name, m.role, u.status, m.created_at
		FROM tenant_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $2 OFFSET $3`,
		ac.TenantID, pageSize, offset(page, pageSize))
	if err != nil {
		return nil, 0, scanErr(err, "list members")
	}
	defer rows.Close()
	var out []MemberView
	for rows.Next() {
		var v MemberView
		if err := rows.Scan(&v.ID, &v.UserID, &v.Email, &v.DisplayName, &v.Role, &v.Status, &v.CreatedAt); err != nil {
			return nil, 0, scanErr(err, "scan member")
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// Membership loads the role of a user within a tenant (no request scope:
// used by auth before a tenant context exists).
func (r *Members) Membership(ctx context.Context, tenantID, userID int64) (string, error) {
	var role string
	err := r.Q.QueryRowContext(ctx, `
		SELECT role FROM tenant_members WHERE tenant_id = $1 AND user_id = $2`,
		tenantID, userID).Scan(&role)
	if err != nil {
		return "", RowNotFound(err)
	}
	return role, nil
}

// ByID loads one membership row of the scoped tenant.
func (r *Members) ByID(ctx context.Context, memberID int64) (*model.TenantMember, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	m := &model.TenantMember{}
	err = r.Q.QueryRowContext(ctx, `
		SELECT id, tenant_id, user_id, role, created_at
		FROM tenant_members
		WHERE tenant_id = $1 AND id = $2`,
		ac.TenantID, memberID).Scan(&m.ID, &m.TenantID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return nil, RowNotFound(scanErr(err, "member"))
	}
	return m, nil
}

// UpdateRole changes the role of a member within the scoped tenant.
func (r *Members) UpdateRole(ctx context.Context, memberID int64, role string) error {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return err
	}
	res, err := r.Q.ExecContext(ctx, `
		UPDATE tenant_members SET role = $1
		WHERE tenant_id = $2 AND id = $3`, role, ac.TenantID, memberID)
	if err != nil {
		return scanErr(err, "update member role")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a membership from the scoped tenant.
func (r *Members) Delete(ctx context.Context, memberID int64) error {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return err
	}
	res, err := r.Q.ExecContext(ctx, `
		DELETE FROM tenant_members WHERE tenant_id = $1 AND id = $2`,
		ac.TenantID, memberID)
	if err != nil {
		return scanErr(err, "delete member")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TenantsOfUser lists the tenants a user joined with their roles (login
// response, issue #3).
func (r *Members) TenantsOfUser(ctx context.Context, userID int64) ([]TenantBrief, error) {
	rows, err := r.Q.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug, m.role
		FROM tenant_members m
		JOIN tenants t ON t.id = m.tenant_id
		WHERE m.user_id = $1 AND t.status = 'active'
		ORDER BY t.id`, userID)
	if err != nil {
		return nil, scanErr(err, "tenants of user")
	}
	defer rows.Close()
	var out []TenantBrief
	for rows.Next() {
		var v TenantBrief
		if err := rows.Scan(&v.ID, &v.Name, &v.Slug, &v.Role); err != nil {
			return nil, scanErr(err, "scan tenant view")
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// TenantBrief is the login projection of one joinable tenant.
type TenantBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role"`
}

// MemberIDsByRoles returns user ids of members with any of the given roles
// in a tenant (notification recipients, issue #7).
func (r *Members) MemberIDsByRoles(ctx context.Context, tenantID int64, roles ...string) ([]int64, error) {
	rows, err := r.Q.QueryContext(ctx, `
		SELECT user_id FROM tenant_members
		WHERE tenant_id = $1 AND role = ANY($2)`, tenantID, roles)
	if err != nil {
		return nil, scanErr(err, "members by role")
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, scanErr(err, "scan member id")
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AgentsAndAdmins returns recipients for the ticket.created event.
func (r *Members) AgentsAndAdmins(ctx context.Context, tenantID int64) ([]int64, error) {
	return r.MemberIDsByRoles(ctx, tenantID, model.RoleAgent, model.RoleTenantAdmin)
}

func offset(page, pageSize int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * pageSize
}
