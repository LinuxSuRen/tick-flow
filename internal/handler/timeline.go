package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Timeline wires comments and timeline reads (issue #14).
type Timeline struct {
	Svc *service.TimelineService
}

// Register mounts timeline routes.
func (h *Timeline) Register(rg *gin.RouterGroup) {
	rg.POST("/tickets/:id/comments", h.comment)
	rg.GET("/tickets/:id/timeline", h.list)
}

func (h *Timeline) comment(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		Body       string `json:"body"`
		IsInternal bool   `json:"is_internal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		return
	}
	view, err := h.Svc.AddComment(c.Request.Context(), id, req.Body, req.IsInternal)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": view})
}

func (h *Timeline) list(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	page, pageSize := paginationOf(c)
	items, total, err := h.Svc.List(c.Request.Context(), id, page, pageSize)
	if err != nil {
		mapError(c, err)
		return
	}
	httpx.Page(c, items, page, pageSize, total)
}
