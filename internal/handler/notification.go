package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Notification wires notification reads (issue #16).
type Notification struct {
	Svc *service.NotificationService
}

// Register mounts notification routes.
func (h *Notification) Register(rg *gin.RouterGroup) {
	rg.GET("/notifications", h.list)
	rg.POST("/notifications/read", h.markRead)
}

func (h *Notification) list(c *gin.Context) {
	page, pageSize := paginationOf(c)
	unreadOnly := c.Query("unread_only") == "true"
	items, total, err := h.Svc.List(c.Request.Context(), unreadOnly, page, pageSize)
	if err != nil {
		mapError(c, err)
		return
	}
	httpx.Page(c, items, page, pageSize, total)
}

func (h *Notification) markRead(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	// An empty body is the mark-all form (issue #3: ids defaults to all).
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
			return
		}
	}
	if _, err := h.Svc.MarkRead(c.Request.Context(), req.IDs); err != nil {
		mapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
