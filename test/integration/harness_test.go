// Package integration runs the full HTTP stack against a throwaway
// PostgreSQL container (testcontainers). Cases cover the acceptance lists
// of issues #11 through #16: auth flows, tenant isolation, the permission
// matrix, the state machine, timeline visibility, attachments and
// notifications.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LinuxSuRen/tick-flow/internal/app"
	"github.com/LinuxSuRen/tick-flow/internal/config"
	"github.com/LinuxSuRen/tick-flow/migrations"
)

var testApp *app.App

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// Keep server logs visible while diagnosing; flip to io.Discard once
	// the suite is stable.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("tickflow"),
		postgres.WithUsername("tickflow"),
		postgres.WithPassword("tickflow"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = pg.Terminate(ctx) }()

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "container dsn: %v\n", err)
		os.Exit(1)
	}

	uploadDir, err := os.MkdirTemp("", "tickflow-uploads-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp upload dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(uploadDir)

	testApp, err = app.New(config.Config{
		DBDSN:      dsn,
		JWTSecret:  "integration-test-secret",
		UploadDir:  uploadDir,
		UploadMax:  1024, // 1KB keeps the oversize case cheap
		UploadExts: []string{".png", ".jpg", ".jpeg", ".webp", ".gif", ".pdf", ".zip", ".txt"},
	}, migrations.FS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wire app: %v\n", err)
		os.Exit(1)
	}
	defer testApp.Close()

	os.Exit(m.Run())
}

// reset drops all data between cases.
func reset(t *testing.T) {
	t.Helper()
	if err := testApp.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
}

// client issues authenticated requests.
type client struct {
	t     *testing.T
	token string
}

func (c *client) do(method, path string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case io.Reader:
		reader = b
	case string:
		reader = strings.NewReader(b)
	default:
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	w := httptest.NewRecorder()
	testApp.Engine.ServeHTTP(w, req)
	return w
}

func (c *client) doMultipart(path, field, filename, contentType string, content []byte) *httptest.ResponseRecorder {
	c.t.Helper()
	body := &bytes.Buffer{}
	writer := newMultipartWriter(body, field, filename, contentType, content)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	w := httptest.NewRecorder()
	testApp.Engine.ServeHTTP(w, req)
	return w
}

func (c *client) mustDo(method, path string, body any, wantStatus int) *httptest.ResponseRecorder {
	c.t.Helper()
	w := c.do(method, path, body)
	if w.Code != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d, body %s", method, path, w.Code, wantStatus, w.Body.String())
	}
	return w
}

// dataOf unwraps {data}.
func dataOf(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode data envelope: %v, body %s", err, w.Body.String())
	}
	return env.Data
}

// pageOf unwraps the pagination envelope.
func pageOf(t *testing.T, w *httptest.ResponseRecorder) (items []map[string]any, total float64) {
	t.Helper()
	var env struct {
		Data  []map[string]any `json:"data"`
		Total float64          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode page envelope: %v, body %s", err, w.Body.String())
	}
	return env.Data, env.Total
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v, body %s", err, w.Body.String())
	}
	return env.Error.Code
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func num(m map[string]any, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// setupTenant boots a fresh tenant with an admin and returns the admin
// client plus the tenant id. Seed packs come with it (issue #19).
func setupTenant(t *testing.T, slug string) (*client, int64) {
	t.Helper()
	w := (&client{t: t}).do("POST", "/api/auth/setup", map[string]any{
		"tenant_name":    "租户-" + slug,
		"slug":           slug,
		"admin_email":    "admin@" + slug + ".test",
		"admin_password": "password123",
		"admin_name":     "管理员" + slug,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("setup tenant: status %d body %s", w.Code, w.Body.String())
	}
	data := dataOf(t, w)
	token := str(data, "token")
	me := data["me"].(map[string]any)
	tenant := me["tenant"].(map[string]any)
	return &client{t: t, token: token}, int64(tenant["id"].(float64))
}

// loginAs returns a tenant-scoped client for an existing activated
// account (invite → accept → login → select).
func loginAs(t *testing.T, admin *client, email, role, password string) *client {
	t.Helper()
	w := admin.mustDo("POST", "/api/tenants/current/members", map[string]any{
		"email": email, "role": role,
	}, http.StatusCreated)
	token := str(dataOf(t, w), "invite_token")

	anon := &client{t: t}
	anon.mustDo("POST", "/api/auth/accept-invitation", map[string]any{
		"token": token, "password": password,
	}, http.StatusNoContent)

	w = anon.mustDo("POST", "/api/auth/login", map[string]any{
		"email": email, "password": password,
	}, http.StatusOK)
	data := dataOf(t, w)
	me := data["me"].(map[string]any)
	tenants := me["tenants"].([]any)
	if len(tenants) == 0 {
		t.Fatal("expected one joinable tenant")
	}
	tenantID := int64(tenants[0].(map[string]any)["id"].(float64))

	logged := &client{t: t, token: str(data, "token")}
	w = logged.mustDo("POST", "/api/auth/select-tenant", map[string]any{
		"tenant_id": tenantID,
	}, http.StatusOK)
	return &client{t: t, token: str(dataOf(t, w), "token")}
}

// createTicket posts a valid IT support ticket.
func createTicket(t *testing.T, c *client, typeID int64, title string) map[string]any {
	t.Helper()
	return createTicketWith(t, c, typeID, title, map[string]any{
		"category":     "hardware",
		"impact_scope": "single",
		"urgency":      "normal",
		"description":  "集成测试描述",
	})
}

// createDeviceTicket posts a valid device repair ticket (photo uploads are
// exercised separately).
func createDeviceTicket(t *testing.T, c *client, typeID int64, title string) map[string]any {
	t.Helper()
	return createTicketWith(t, c, typeID, title, map[string]any{
		"device_no":       "DEV-001",
		"device_location": "3 号楼一层",
		"fault_category":  "hardware",
		"occurred_at":     "2026-11-14T09:30:00Z",
	})
}

func createTicketWith(t *testing.T, c *client, typeID int64, title string, fields map[string]any) map[string]any {
	t.Helper()
	w := c.mustDo("POST", "/api/tickets", map[string]any{
		"ticket_type_id": typeID,
		"title":          title,
		"priority":       "normal",
		"custom_fields":  fields,
	}, http.StatusCreated)
	return dataOf(t, w)
}

// firstTypeID resolves the seeded it-support type.
func firstTypeID(t *testing.T, c *client, slug string) int64 {
	t.Helper()
	w := c.mustDo("GET", "/api/ticket-types?page_size=100", nil, http.StatusOK)
	items, _ := pageOf(t, w)
	for _, it := range items {
		if str(it, "slug") == slug && str(it, "status") == "enabled" {
			return num(it, "id")
		}
	}
	t.Fatalf("seed type %s not found", slug)
	return 0
}

func tempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
