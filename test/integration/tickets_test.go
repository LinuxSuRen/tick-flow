package integration

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/LinuxSuRen/tick-flow/internal/middleware"
)

// TestTenantIsolation drives the multi-tenant red line (issue #6): tenant
// A tokens must get 404 for every tenant B resource. Tenant B is created
// through SQL because P0 exposes no tenant-creation API beyond setup.
func TestTenantIsolation(t *testing.T) {
	reset(t)
	adminA, _ := setupTenant(t, "iso-a")

	// tenant B, straight into the database
	var tenantB, userB int64
	err := testApp.DB.QueryRow(`INSERT INTO tenants (name, slug) VALUES ('租户B', 'iso-b') RETURNING id`).Scan(&tenantB)
	if err != nil {
		t.Fatal(err)
	}
	err = testApp.DB.QueryRow(`
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('admin@iso-b.test', 'x', '管理员B') RETURNING id`).Scan(&userB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testApp.DB.Exec(`
		INSERT INTO tenant_members (tenant_id, user_id, role) VALUES ($1, $2, 'tenant_admin')`,
		tenantB, userB); err != nil {
		t.Fatal(err)
	}
	tokenB, err := middleware.IssueToken(testApp.Config.JWTSecret, userB, tenantB, "tenant_admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	adminB := &client{t: t, token: tokenB}

	// a ticket in A
	typeA := firstTypeID(t, adminA, "it-support")
	ticketA := createTicket(t, adminA, typeA, "租户 A 的工单")
	idA := num(ticketA, "id")

	w := adminA.doMultipart("/api/tickets/"+itoa(idA)+"/attachments", "file", "a.png", "image/png", pngBytes())
	if w.Code != http.StatusCreated {
		t.Fatalf("upload in A: %d %s", w.Code, w.Body.String())
	}
	attA := dataOf(t, w)

	// tenant B tries to touch A's ticket, type, attachment and members
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/tickets/" + itoa(idA), nil},
		{"GET", "/api/ticket-types/" + itoa(typeA), nil},
		{"GET", "/api/attachments/" + itoa(num(attA, "id")) + "/content", nil},
		{"POST", "/api/tickets/" + itoa(idA) + "/comments", map[string]any{"body": "越权回复"}},
		{"POST", "/api/tickets/" + itoa(idA) + "/transitions", map[string]any{"action": "accept"}},
	} {
		w := adminB.do(tc.method, tc.path, tc.body)
		if w.Code != http.StatusNotFound || errCode(t, w) != "NOT_FOUND" {
			t.Fatalf("%s %s cross-tenant = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// B never sees A's tickets in the list
	w = adminB.mustDo("GET", "/api/tickets", nil, http.StatusOK)
	items, total := pageOf(t, w)
	if total != 0 || len(items) != 0 {
		t.Fatalf("tenant B sees %v foreign tickets", total)
	}
}

// TestPermissionMatrix walks the ✗ cells of the matrix (issue #6).
func TestPermissionMatrix(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "perm")
	requester := loginAs(t, admin, "req@perm.test", "requester", "password123")
	agent := loginAs(t, admin, "ag@perm.test", "agent", "password123")

	typeID := firstTypeID(t, admin, "it-support")
	ticket := createTicket(t, requester, typeID, "权限测试工单")
	id := itoa(num(ticket, "id"))

	// requester: internal note forbidden
	w := requester.mustDo("POST", "/api/tickets/"+id+"/comments", map[string]any{
		"body": "内部备注", "is_internal": true,
	}, http.StatusForbidden)
	if errCode(t, w) != "FORBIDDEN" {
		t.Fatalf("requester internal note code = %s", errCode(t, w))
	}

	// requester: assignment forbidden
	w = requester.do("POST", "/api/tickets/"+id+"/assignments", map[string]any{"assignee_id": num(dataOf(t, agent.mustDo("GET", "/api/me", nil, http.StatusOK)), "id")})
	if w.Code != http.StatusForbidden {
		t.Fatalf("requester assignment = %d", w.Code)
	}

	// requester: member management / type management / branding forbidden
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/tenants/current/members", nil},
		{"POST", "/api/tenants/current/members", map[string]any{"email": "x@perm.test", "role": "agent"}},
		{"POST", "/api/ticket-types", map[string]any{"slug": "rogue", "name": "越权类型"}},
		{"PATCH", "/api/tenants/current", map[string]any{"name": "越权重命名"}},
	} {
		w := requester.do(tc.method, tc.path, tc.body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("requester %s %s = %d", tc.method, tc.path, w.Code)
		}
	}

	// agent: member management and branding forbidden
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/tenants/current/members", nil},
		{"PATCH", "/api/tenants/current", map[string]any{"name": "越权"}},
	} {
		w := agent.do(tc.method, tc.path, tc.body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("agent %s %s = %d", tc.method, tc.path, w.Code)
		}
	}

	// requester cannot read someone else's ticket (404, not 403)
	other := createTicket(t, admin, typeID, "管理员自己的工单")
	w = requester.do("GET", "/api/tickets/"+itoa(num(other, "id")), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("requester reading foreign ticket = %d", w.Code)
	}
}

// TestTicketStateMachine walks the happy path and every illegal move
// (issue #13).
func TestTicketStateMachine(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "state")
	requester := loginAs(t, admin, "req@state.test", "requester", "password123")

	typeID := firstTypeID(t, admin, "it-support")
	ticket := createTicket(t, requester, typeID, "状态机测试")
	id := itoa(num(ticket, "id"))

	// initial state comes from the workflow
	if str(ticket, "state") != "pending" || str(ticket, "state_name") != "待受理" {
		t.Fatalf("initial state = %s/%s", str(ticket, "state"), str(ticket, "state_name"))
	}

	// validation failures carry field errors
	w := requester.do("POST", "/api/tickets", map[string]any{
		"ticket_type_id": typeID, "title": "缺字段",
		"custom_fields":  map[string]any{"category": "hardware"},
	})
	if w.Code != http.StatusBadRequest || errCode(t, w) != "VALIDATION_FAILED" {
		t.Fatalf("missing fields accepted: %d %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "description", "impact_scope", "urgency") {
		t.Fatalf("field errors missing required keys: %s", w.Body.String())
	}

	// requester cannot accept (roles: agent, tenant_admin)
	w = requester.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{
		"action": "accept",
	}, http.StatusForbidden)

	// unknown action in state
	w = admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{
		"action": "teleport",
	}, http.StatusConflict)
	if errCode(t, w) != "STATE_CONFLICT" {
		t.Fatalf("unknown action code = %s", errCode(t, w))
	}

	// full path: accept → resolve → close → reopen → close
	admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "accept"}, http.StatusOK)
	admin.mustDo("POST", "/api/tickets/"+id+"/comments", map[string]any{
		"body": "已定位问题", "is_internal": true,
	}, http.StatusCreated)
	admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "resolve"}, http.StatusOK)
	closed := dataOf(t, requester.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{
		"action": "close",
	}, http.StatusOK))
	if str(closed, "state") != "closed" || closed["closed_at"] == nil {
		t.Fatalf("closed ticket state = %v closed_at = %v", closed["state"], closed["closed_at"])
	}
	reopened := dataOf(t, requester.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{
		"action": "reopen",
	}, http.StatusOK))
	if str(reopened, "state") != "pending" || reopened["closed_at"] != nil {
		t.Fatalf("reopen state = %v", reopened["state"])
	}
	admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "accept"}, http.StatusOK)
	admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "resolve"}, http.StatusOK)
	requester.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "close"}, http.StatusOK)

	// transitions listing reflects the current state and role
	detail := dataOf(t, admin.mustDo("GET", "/api/tickets/"+id, nil, http.StatusOK))
	transitions := detail["transitions"].([]any)
	if len(transitions) != 1 {
		t.Fatalf("closed ticket should expose only reopen, got %d", len(transitions))
	}

	// requester list is narrowed to own tickets
	w = requester.mustDo("GET", "/api/tickets", nil, http.StatusOK)
	_, total := pageOf(t, w)
	if total != 1 {
		t.Fatalf("requester sees %v tickets, want 1", total)
	}

	// assignment round trip
	_ = loginAs(t, admin, "ag@state.test", "agent", "password123")
	assigned := dataOf(t, admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{
		"action": "reopen",
	}, http.StatusOK))
	_ = assigned
	w = admin.do("POST", "/api/tickets/"+id+"/assignments", map[string]any{
		"assignee_id": -1,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid assignee = %d", w.Code)
	}
}

// TestConcurrentTicketNumbers hammers creation in parallel; numbers must
// stay unique per tenant (issue #13).
func TestConcurrentTicketNumbers(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "numbers")
	requester := loginAs(t, admin, "req@numbers.test", "requester", "password123")
	typeID := firstTypeID(t, admin, "it-support")

	const n = 20
	var wg sync.WaitGroup
	numbers := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tk := createTicket(t, requester, typeID, "并发工单")
			numbers[i] = str(tk, "number")
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for _, no := range numbers {
		if no == "" || seen[no] {
			t.Fatalf("duplicate or empty ticket number %q", no)
		}
		seen[no] = true
	}
}

func pngBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
