package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// TicketType wires template pack management (issue #12).
type TicketType struct {
	Svc *service.TicketTypeService
}

// Register mounts ticket type routes; reads for everyone logged in,
// mutations for tenant_admin.
func (h *TicketType) Register(rg *gin.RouterGroup) {
	rg.GET("/ticket-types", h.list)
	rg.GET("/ticket-types/:id", h.get)

	admin := rg.Group("", middleware.RequireRole(model.RoleTenantAdmin))
	admin.POST("/ticket-types", h.create)
	admin.PUT("/ticket-types/:id", h.update)
	admin.POST("/ticket-types/:id/disable", h.disable)
}

func (h *TicketType) list(c *gin.Context) {
	page, pageSize := paginationOf(c)
	items, total, err := h.Svc.ListAll(c.Request.Context(), page, pageSize)
	if err != nil {
		mapError(c, err)
		return
	}
	httpx.Page(c, items, page, pageSize, total)
}

func (h *TicketType) get(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	view, err := h.Svc.Get(c.Request.Context(), id)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

func (h *TicketType) create(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.ErrInternal(c, err)
		return
	}
	view, err := h.Svc.Create(c.Request.Context(), raw)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": view})
}

func (h *TicketType) update(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.ErrInternal(c, err)
		return
	}
	view, err := h.Svc.Update(c.Request.Context(), id, raw)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

func (h *TicketType) disable(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "status must be enabled or disabled")
		return
	}
	view, err := h.Svc.SetStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}
