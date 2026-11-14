// Package router assembles the Gin engine: global middlewares, the public
// health endpoint, and the /api group with its authenticated sub-groups.
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
}

// New builds the engine. Route registrations grow per implementation issue;
// the skeleton wires the observability middlewares, healthz and the /api group.
func New(deps Deps) *gin.Engine {
	engine := gin.New()
	engine.Use(
		middleware.RequestID(),
		middleware.AccessLog(),
		middleware.Recovery(),
	)

	(&handler.Health{DB: deps.DB}).Register(&engine.RouterGroup)

	api := engine.Group("/api")
	api.Use(middleware.RequireAuth(deps.JWTSecret))
	{
		// Authenticated route groups (me, tenants, ticket types, tickets,
		// notifications, attachments) are registered by their handlers in the
		// domain batches.
		_ = api
	}

	return engine
}
