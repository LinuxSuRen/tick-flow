package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Ticket wires ticket creation, listing, detail, transitions and
// assignment (issue #13).
type Ticket struct {
	Svc *service.TicketService
}

// Register mounts ticket routes.
func (h *Ticket) Register(rg *gin.RouterGroup) {
	rg.GET("/tickets", h.list)
	rg.POST("/tickets", h.create)
	rg.GET("/tickets/:id", h.get)
	rg.POST("/tickets/:id/transitions", h.transition)
	rg.POST("/tickets/:id/assignments", middleware.RequireRole(model.RoleAgent, model.RoleTenantAdmin), h.assign)
}

func (h *Ticket) list(c *gin.Context) {
	page, pageSize := paginationOf(c)
	filter := repository.TicketFilter{
		State:    c.Query("state"),
		Priority: c.Query("priority"),
		Keyword:  c.Query("keyword"),
	}
	if v := c.Query("type"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.TypeID = &n
		}
	}
	if v := c.Query("assignee"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Assignee = &n
		}
	}
	items, total, err := h.Svc.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		mapError(c, err)
		return
	}
	httpx.Page(c, items, page, pageSize, total)
}

func (h *Ticket) create(c *gin.Context) {
	var req service.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		return
	}
	detail, err := h.Svc.Create(c.Request.Context(), req)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": detail})
}

func (h *Ticket) get(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	detail, err := h.Svc.Get(c.Request.Context(), id)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

func (h *Ticket) transition(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		Action  string `json:"action"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Action == "" {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "action is required")
		return
	}
	detail, err := h.Svc.Transition(c.Request.Context(), id, req.Action, req.Comment)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

func (h *Ticket) assign(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		AssigneeID int64 `json:"assignee_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AssigneeID <= 0 {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "assignee_id is required")
		return
	}
	detail, err := h.Svc.Assign(c.Request.Context(), id, req.AssigneeID)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}
