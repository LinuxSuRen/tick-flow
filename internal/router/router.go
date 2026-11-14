// Package router assembles the Gin engine: global middlewares, the public
// health endpoint, and the /api groups (public, authenticated, tenant
// scoped).
package router

import (
	"database/sql"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/handler"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
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

	engine.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "route not found"}})
	})
	return engine
}
