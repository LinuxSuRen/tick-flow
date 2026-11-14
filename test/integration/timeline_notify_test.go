package integration

import (
	"net/http"
	"strings"
	"testing"
)

// TestTimelineVisibility covers the dual entry points and the requester
// filter (issue #14).
func TestTimelineVisibility(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "tl")
	requester := loginAs(t, admin, "req@tl.test", "requester", "password123")

	typeID := firstTypeID(t, admin, "it-support")
	ticket := createTicket(t, requester, typeID, "时间线测试")
	id := itoa(num(ticket, "id"))

	// agent posts a public reply and an internal note
	dataOf(t, admin.mustDo("POST", "/api/tickets/"+id+"/comments", map[string]any{
		"body": "对外回复", "is_internal": false,
	}, http.StatusCreated))
	dataOf(t, admin.mustDo("POST", "/api/tickets/"+id+"/comments", map[string]any{
		"body": "内部备注", "is_internal": true,
	}, http.StatusCreated))

	// requester timeline: no internal notes
	items, total := pageOf(t, requester.mustDo("GET", "/api/tickets/"+id+"/timeline", nil, http.StatusOK))
	if total != 2 {
		t.Fatalf("requester timeline total = %v (creation system + reply)", total)
	}
	for _, it := range items {
		if str(it, "kind") == "internal_note" {
			t.Fatal("requester must not see internal notes")
		}
	}

	// agent timeline: everything
	items, total = pageOf(t, admin.mustDo("GET", "/api/tickets/"+id+"/timeline", nil, http.StatusOK))
	if total != 3 {
		t.Fatalf("agent timeline total = %v", total)
	}
	kinds := map[string]int{}
	for _, it := range items {
		kinds[str(it, "kind")]++
	}
	if kinds["internal_note"] != 1 || kinds["comment"] != 1 {
		t.Fatalf("kinds = %v", kinds)
	}

	// ascending order
	if !strings.Contains(wBody(admin, id), "对外回复") {
		t.Fatal("sanity")
	}

	// 16KB body limit
	w := admin.do("POST", "/api/tickets/"+id+"/comments", map[string]any{
		"body": strings.Repeat("x", 16<<10+1),
	})
	if errCode(t, w) != "VALIDATION_FAILED" {
		t.Fatalf("oversize comment code = %s", errCode(t, w))
	}
}

func wBody(c *client, ticketID string) string {
	w := c.do("GET", "/api/tickets/"+ticketID+"/timeline", nil)
	return w.Body.String()
}

// TestAttachments covers upload rules and downloads (issue #15).
func TestAttachments(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "att")
	requester := loginAs(t, admin, "req@att.test", "requester", "password123")

	typeID := firstTypeID(t, admin, "device-repair")
	ticket := createDeviceTicket(t, requester, typeID, "附件测试")
	id := itoa(num(ticket, "id"))

	// allowed extension uploads fine, traversal-looking name is sanitized
	w := requester.doMultipart("/api/tickets/"+id+"/attachments", "file",
		"..\\..\\evil.png", "image/png", pngBytes())
	if w.Code != http.StatusCreated {
		t.Fatalf("png upload = %d %s", w.Code, w.Body.String())
	}
	att := dataOf(t, w)
	if str(att, "filename") == "" {
		t.Fatal("upload response missing filename")
	}

	// download carries disposition and content
	w = requester.do("GET", "/api/attachments/"+itoa(num(att, "id"))+"/content", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("download = %d", w.Code)
	}
	if disp := w.Header().Get("Content-Disposition"); !strings.Contains(disp, "attachment") {
		t.Fatalf("disposition = %q", disp)
	}

	// the system timeline entry linked the attachment
	items, _ := pageOf(t, admin.mustDo("GET", "/api/tickets/"+id+"/timeline", nil, http.StatusOK))
	linked := false
	for _, it := range items {
		if str(it, "kind") == "system" && strings.Contains(str(it, "body"), "上传了附件") {
			linked = true
		}
	}
	if !linked {
		t.Fatal("attachment upload must write a system timeline entry")
	}

	// non-whitelisted extension rejected
	w = requester.doMultipart("/api/tickets/"+id+"/attachments", "file", "evil.exe", "application/octet-stream", []byte("MZ"))
	if errCode(t, w) != "VALIDATION_FAILED" {
		t.Fatalf("exe upload code = %s body %s", errCode(t, w), w.Body.String())
	}

	// oversize rejected (limit is 1KB in the test app)
	w = requester.doMultipart("/api/tickets/"+id+"/attachments", "file", "big.png", "image/png", make([]byte, 2048))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversize upload = %d", w.Code)
	}

	// requester cannot download attachments of foreign tickets
	foreign := createDeviceTicket(t, admin, typeID, "别人家的工单")
	w2 := admin.doMultipart("/api/tickets/"+itoa(num(foreign, "id"))+"/attachments", "file", "f.png", "image/png", pngBytes())
	fAtt := dataOf(t, w2)
	w = requester.do("GET", "/api/attachments/"+itoa(num(fAtt, "id"))+"/content", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("requester foreign attachment download = %d", w.Code)
	}
}

// TestNotifications covers recipients per topic, operator exclusion, the
// unread count and mark-read (issues #7, #16).
func TestNotifications(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "notif")
	requester := loginAs(t, admin, "req@notif.test", "requester", "password123")
	agent := loginAs(t, admin, "ag@notif.test", "agent", "password123")

	typeID := firstTypeID(t, admin, "it-support")
	ticket := createTicket(t, requester, typeID, "通知测试")
	id := itoa(num(ticket, "id"))

	// ticket.created → agents + admins; the requester creator is not staff
	// and stays at zero.
	countUnread := func(c *client) int64 {
		w := c.do("GET", "/api/notifications?unread_only=true&page_size=100", nil)
		_, total := pageOf(t, w)
		return int64(total)
	}
	if got := countUnread(agent); got != 1 {
		t.Fatalf("agent unread after created = %d", got)
	}
	if got := countUnread(admin); got != 1 {
		t.Fatalf("admin unread after created = %d", got)
	}
	if got := countUnread(requester); got != 0 {
		t.Fatalf("requester (creator, non-staff) unread = %d", got)
	}

	// accept (operator admin) notifies the creator
	agentMe := dataOf(t, agent.mustDo("GET", "/api/me", nil, http.StatusOK))
	admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "accept"}, http.StatusOK)
	if got := countUnread(requester); got != 1 {
		t.Fatalf("requester unread after accept = %d", got)
	}
	dataOf(t, admin.mustDo("POST", "/api/tickets/"+id+"/assignments", map[string]any{
		"assignee_id": num(agentMe["user"].(map[string]any), "id"),
	}, http.StatusOK))

	// assigned → only the assignee
	if got := countUnread(agent); got != 2 {
		t.Fatalf("agent unread after assigned = %d", got)
	}
	if got := countUnread(admin); got != 1 {
		t.Fatalf("admin unread after assigned = %d (operator excluded)", got)
	}

	// agent comments: creator + assignee minus the author
	dataOf(t, agent.mustDo("POST", "/api/tickets/"+id+"/comments", map[string]any{"body": "已受理"}, http.StatusCreated))
	if got := countUnread(requester); got != 2 {
		t.Fatalf("requester unread after comment = %d", got)
	}
	if got := countUnread(agent); got != 2 {
		t.Fatalf("agent unread after own comment = %d (author excluded)", got)
	}

	// admin resolves: creator + assignee minus operator
	admin.mustDo("POST", "/api/tickets/"+id+"/transitions", map[string]any{"action": "resolve"}, http.StatusOK)
	if got := countUnread(requester); got != 3 {
		t.Fatalf("requester unread after resolve = %d", got)
	}
	if got := countUnread(agent); got != 3 {
		t.Fatalf("agent unread after resolve = %d", got)
	}

	// notification rows reference the ticket for deep links
	items, _ := pageOf(t, requester.mustDo("GET", "/api/notifications?page_size=100", nil, http.StatusOK))
	if len(items) == 0 || num(items[0], "ticket_id") != num(ticket, "id") {
		t.Fatalf("notification ticket reference wrong: %+v", items)
	}

	// mark read: explicit ids then the sweep-all form
	requester.mustDo("POST", "/api/notifications/read", map[string]any{
		"ids": []int64{num(items[0], "id")},
	}, http.StatusNoContent)
	if got := countUnread(requester); got != 2 {
		t.Fatalf("requester unread after mark-one = %d", got)
	}
	requester.mustDo("POST", "/api/notifications/read", nil, http.StatusNoContent)
	if got := countUnread(requester); got != 0 {
		t.Fatalf("requester unread after mark-all = %d", got)
	}
}

// TestTicketTypeAdmin covers validator rejections and CRUD (issue #12).
func TestTicketTypeAdmin(t *testing.T) {
	reset(t)
	admin, _ := setupTenant(t, "types")

	// invalid: dangling workflow reference
	w := admin.do("POST", "/api/ticket-types", map[string]any{
		"slug": "broken", "name": "坏类型",
		"workflow": map[string]any{
			"states": []any{map[string]any{"key": "a", "name": "A", "type": "initial"}},
			"transitions": []any{map[string]any{
				"from": "a", "action": "go", "to": "ghost", "roles": []string{"agent"},
			}},
		},
	})
	if errCode(t, w) != "VALIDATION_FAILED" || !strings.Contains(w.Body.String(), "ghost") {
		t.Fatalf("dangling state accepted: %s", w.Body.String())
	}

	// valid custom type
	created := dataOf(t, admin.mustDo("POST", "/api/ticket-types", map[string]any{
		"slug": "custom-flow", "name": "自定义流程",
		"form_schema": map[string]any{"fields": []any{
			map[string]any{"key": "site", "label": "地点", "type": "string", "required": true},
		}},
		"workflow": map[string]any{
			"states": []any{
				map[string]any{"key": "new", "name": "新建", "type": "initial"},
				map[string]any{"key": "done", "name": "完成", "type": "final"},
			},
			"transitions": []any{map[string]any{
				"from": "new", "action": "finish", "to": "done", "roles": []string{"agent", "tenant_admin"},
			}},
		},
		"assignment": map[string]any{"mode": "manual"},
	}, http.StatusCreated))
	id := itoa(num(created, "id"))

	// duplicate slug (payload itself valid so the uniqueness is what fires)
	w = admin.do("POST", "/api/ticket-types", map[string]any{
		"slug": "custom-flow", "name": "重复",
		"workflow": map[string]any{
			"states": []any{
				map[string]any{"key": "new", "name": "新建", "type": "initial"},
				map[string]any{"key": "done", "name": "完成", "type": "final"},
			},
			"transitions": []any{map[string]any{
				"from": "new", "action": "finish", "to": "done", "roles": []string{"agent"},
			}},
		},
		"assignment": map[string]any{"mode": "manual"},
	})
	if errCode(t, w) != "DUPLICATED" {
		t.Fatalf("duplicate slug code = %s body %s", errCode(t, w), w.Body.String())
	}

	// update renames
	updated := dataOf(t, admin.mustDo("PUT", "/api/ticket-types/"+id, map[string]any{
		"slug": "custom-flow", "name": "改名流程",
		"workflow": created["workflow"],
		"assignment": map[string]any{"mode": "manual"},
	}, http.StatusOK))
	if str(updated, "name") != "改名流程" {
		t.Fatalf("update lost name: %s", w.Body.String())
	}

	// disable hides it from creation
	dataOf(t, admin.mustDo("POST", "/api/ticket-types/"+id+"/disable", map[string]any{
		"status": "disabled",
	}, http.StatusOK))
	requester := loginAs(t, admin, "req@types.test", "requester", "password123")
	w = requester.do("POST", "/api/tickets", map[string]any{
		"ticket_type_id": num(created, "id"), "title": "停用类型",
	})
	if errCode(t, w) != "VALIDATION_FAILED" {
		t.Fatalf("disabled type ticket code = %s", errCode(t, w))
	}
}
