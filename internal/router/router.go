// Package router assembles the Gin engine: global middlewares, the public
// health endpoint, the /api groups (public, authenticated, tenant scoped)
// and the embedded SPA.
package router

import (
	"database/sql"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/handler"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
	webfs "github.com/LinuxSuRen/tick-flow/web"
)

// Deps groups the router's collaborators.
type Deps struct {
	DB        *sql.DB
	JWTSecret string

	Auth         *handler.Auth
	Tenant       *handler.Tenant
	Member       *handler.Member
	TicketType   *handler.TicketType
	Ticket       *handler.Ticket
	Timeline     *handler.Timeline
	Attachment   *handler.Attachment
	Notification *handler.Notification
}

// New builds the engine with every route registered. Middleware chain per
// #6: RequireAuth then RequireTenant for business routes; RequireRole on
// top where the permission matrix demands it.
func New(deps Deps) *gin.Engine {
	engine := gin.New()
	engine.Use(
		middleware.RequestID(),
		middleware.AccessLog(),
		middleware.Recovery(),
	)

	(&handler.Health{DB: deps.DB}).Register(&engine.RouterGroup)

	api := engine.Group("/api")

	// Public: setup, login and invitation acceptance.
	deps.Auth.RegisterPublic(api.Group(""))

	// Authenticated but possibly tenant-free: me and tenant selection.
	authed := api.Group("", middleware.RequireAuth(deps.JWTSecret))
	deps.Auth.RegisterAuthed(authed)

	// Tenant scoped business routes.
	tenantAPI := api.Group("", middleware.RequireAuth(deps.JWTSecret), middleware.RequireTenant())
	deps.Tenant.Register(tenantAPI)
	deps.Member.Register(tenantAPI)
	deps.TicketType.Register(tenantAPI)
	deps.Ticket.Register(tenantAPI)
	deps.Timeline.Register(tenantAPI)
	deps.Attachment.Register(tenantAPI)
	deps.Notification.Register(tenantAPI)

	registerSPA(engine)
	return engine
}

// registerSPA serves the embedded frontend: hashed assets are immutable,
// everything else falls back to index.html for client-side routing.
func registerSPA(engine *gin.Engine) {
	dist, err := fs.Sub(webfs.Assets, "dist")
	if err != nil {
		return
	}
	index, indexErr := fs.ReadFile(dist, "index.html")

	engine.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{"code": "NOT_FOUND", "message": "route not found"},
			})
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+p), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		data, err := fs.ReadFile(dist, name)
		if err != nil {
			if indexErr != nil {
				c.String(http.StatusNotFound, "frontend not built")
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/html; charset=utf-8", index)
			return
		}
		if name == "index.html" {
			c.Header("Cache-Control", "no-cache")
		} else {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Data(http.StatusOK, contentTypeOf(name), data)
	})
}

func contentTypeOf(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
