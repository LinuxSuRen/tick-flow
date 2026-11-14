// Package handler holds the HTTP handlers: request parsing, response
// shaping. Domain logic lives in internal/service.
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Health responds with the liveness probe including a database ping. It is an
// infrastructure endpoint and intentionally does not use the API error
// envelope.
type Health struct {
	DB interface {
		PingContext(ctx context.Context) error
	}
}

// Register wires GET /healthz.
func (h *Health) Register(rg *gin.RouterGroup) {
	rg.GET("/healthz", h.handle)
}

func (h *Health) handle(c *gin.Context) {
	pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.DB.PingContext(pingCtx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
