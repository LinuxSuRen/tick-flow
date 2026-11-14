package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// TestSetupAuthSeed covers the auth lifecycle and the seed import
// (issues #11, #12, #19).
func TestSetupAuthSeed(t *testing.T) {
	reset(t)
	anon := &client{t: t}

	// setup is one-shot
	w := anon.mustDo("POST", "/api/auth/setup", map[string]any{
		"tenant_name": "演示租户", "slug": "demo",
		"admin_email": "root@demo.test", "admin_password": "password123", "admin_name": "根",
	}, http.StatusCreated)
	data := dataOf(t, w)
	if str(data, "token") == "" {
		t.Fatal("setup must return a token")
	}

	w = anon.mustDo("POST", "/api/auth/setup", map[string]any{
		"tenant_name": "x", "slug": "x", "admin_email": "x@x.test",
		"admin_password": "password123", "admin_name": "x",
	}, http.StatusNotFound)
	if errCode(t, w) != "NOT_FOUND" {
		t.Fatalf("second setup code = %s", errCode(t, w))
	}

	// bad credentials
	w = anon.mustDo("POST", "/api/auth/login", map[string]any{
		"email": "root@demo.test", "password": "wrong-pass",
	}, http.StatusUnauthorized)
	if errCode(t, w) != "UNAUTHORIZED" {
		t.Fatalf("bad login code = %s", errCode(t, w))
	}

	// login → tenant list → select → me
	w = anon.mustDo("POST", "/api/auth/login", map[string]any{
		"email": "root@demo.test", "password": "password123",
	}, http.StatusOK)
	login := dataOf(t, w)
	me := login["me"].(map[string]any)
	if me["tenant"] != nil {
		t.Fatal("login token must not bind a tenant")
	}
	scoped := &client{t: t, token: str(login, "token")}

	w = scoped.mustDo("GET", "/api/me", nil, http.StatusOK)
	if num(dataOf(t, w)["user"].(map[string]any), "id") == 0 {
		t.Fatal("me must expose the user")
	}

	tenants := me["tenants"].([]any)
	tenantID := int64(tenants[0].(map[string]any)["id"].(float64))
	w = scoped.mustDo("POST", "/api/auth/select-tenant", map[string]any{"tenant_id": tenantID}, http.StatusOK)
	selected := dataOf(t, w)
	if selected["me"].(map[string]any)["tenant"] == nil {
		t.Fatal("select-tenant must bind the tenant")
	}

	// seed packs imported and portal-visible
	bound := &client{t: t, token: str(selected, "token")}
	w = bound.mustDo("GET", "/api/ticket-types?page_size=100", nil, http.StatusOK)
	items, total := pageOf(t, w)
	if total < 4 {
		t.Fatalf("expected 4 seed types, got %v", total)
	}
	slugs := map[string]bool{}
	for _, it := range items {
		slugs[str(it, "slug")] = true
		if it["is_builtin"] != true {
			t.Fatalf("seed type %s must be builtin", str(it, "slug"))
		}
	}
	for _, want := range []string{"it-support", "customer-care", "device-repair", "approval"} {
		if !slugs[want] {
			t.Fatalf("seed type %s missing", want)
		}
	}

	// unauthenticated access rejected
	w = anon.do("GET", "/api/ticket-types", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list = %d", w.Code)
	}
}

// TestMemberLifecycle covers invite → activate → role change → removal
// (issue #11).
func TestMemberLifecycle(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "members")

	w := admin.mustDo("POST", "/api/tenants/current/members", map[string]any{
		"email": "agent@members.test", "role": "agent",
	}, http.StatusCreated)
	token := str(dataOf(t, w), "invite_token")
	if token == "" {
		t.Fatal("invite must expose the set-password token to the admin")
	}

	// weak password rejected
	w = admin.do("POST", "/api/auth/accept-invitation", map[string]any{
		"token": token, "password": "short",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("weak password accepted: %d", w.Code)
	}

	anon := &client{t: t}
	anon.mustDo("POST", "/api/auth/accept-invitation", map[string]any{
		"token": token, "password": "password123",
	}, http.StatusNoContent)

	// token is single-use: the account is active now
	w = anon.do("POST", "/api/auth/accept-invitation", map[string]any{
		"token": token, "password": "password123",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("reused invitation accepted: %d", w.Code)
	}

	w = anon.mustDo("POST", "/api/auth/login", map[string]any{
		"email": "agent@members.test", "password": "password123",
	}, http.StatusOK)
	login := dataOf(t, w)
	agent := &client{t: t, token: str(login, "token")}
	w = agent.mustDo("POST", "/api/auth/select-tenant", map[string]any{
		"tenant_id": login["me"].(map[string]any)["tenants"].([]any)[0].(map[string]any)["id"],
	}, http.StatusOK)
	agent = &client{t: t, token: str(dataOf(t, w), "token")}
	if role := dataOf(t, w)["me"].(map[string]any)["tenant"].(map[string]any)["role"]; role != "agent" {
		t.Fatalf("role = %v", role)
	}

	// invite an already-member email duplicates
	w = admin.do("POST", "/api/tenants/current/members", map[string]any{
		"email": "agent@members.test", "role": "agent",
	})
	if errCode(t, w) != "DUPLICATED" {
		t.Fatalf("duplicate member code = %s body %s", errCode(t, w), w.Body.String())
	}

	// member list pages
	w = admin.mustDo("GET", "/api/tenants/current/members", nil, http.StatusOK)
	_, total := pageOf(t, w)
	if total < 2 {
		t.Fatalf("members total = %v", total)
	}

	// change role: find the member row id first
	items, _ := pageOf(t, w)
	var memberID int64
	for _, it := range items {
		if str(it, "email") == "agent@members.test" {
			memberID = num(it, "id")
		}
	}
	if memberID == 0 {
		t.Fatal("member row not found")
	}
	admin.mustDo("PATCH", "/api/tenants/current/members/"+itoa(memberID), map[string]any{
		"role": "requester",
	}, http.StatusOK)

	// tenant branding patch
	w = admin.mustDo("PATCH", "/api/tenants/current", map[string]any{
		"name": "改名租户", "theme": map[string]any{"primary_color": "#123456"},
	}, http.StatusOK)
	if !strings.Contains(w.Body.String(), "改名租户") {
		t.Fatalf("branding update lost: %s", w.Body.String())
	}

	// removal
	admin.mustDo("DELETE", "/api/tenants/current/members/"+itoa(memberID), nil, http.StatusNoContent)
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
