package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Member wires member management (issue #11). All routes are tenant_admin.
type Member struct {
	Svc *service.MemberService
}

// Register mounts member routes.
func (h *Member) Register(rg *gin.RouterGroup) {
	admin := rg.Group("/tenants/current/members", middleware.RequireRole(model.RoleTenantAdmin))
	admin.GET("", h.list)
	admin.POST("", h.invite)
	admin.PATCH("/:id", h.update)
	admin.DELETE("/:id", h.remove)
}

func (h *Member) list(c *gin.Context) {
	page, pageSize := paginationOf(c)
	items, total, err := h.Svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		mapError(c, err)
		return
	}
	httpx.Page(c, items, page, pageSize, total)
}

func (h *Member) invite(c *gin.Context) {
	var req struct {
		Email       string `json:"email"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		return
	}
	result, err := h.Svc.Invite(c.Request.Context(), req.Email, req.Role, req.DisplayName)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"member":       result.Member,
		"invite_token": result.InviteToken,
	}})
}

func (h *Member) update(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Role == "" {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "role is required")
		return
	}
	view, err := h.Svc.UpdateRole(c.Request.Context(), id, req.Role)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

func (h *Member) remove(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.Svc.Remove(c.Request.Context(), id); err != nil {
		mapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func paginationOf(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return normalize(page, pageSize)
}

func normalize(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "id must be a positive integer")
		return 0, false
	}
	return id, true
}
