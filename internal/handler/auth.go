package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Auth wires the authentication endpoints (issue #11).
type Auth struct {
	Svc *service.AuthService
}

// RegisterPublic mounts setup, login and invitation acceptance (no token).
func (h *Auth) RegisterPublic(public *gin.RouterGroup) {
	public.POST("/auth/setup", h.setup)
	public.POST("/auth/login", h.login)
	public.POST("/auth/accept-invitation", h.acceptInvitation)
}

// RegisterAuthed mounts me and tenant selection (token required).
func (h *Auth) RegisterAuthed(authed *gin.RouterGroup) {
	authed.GET("/me", h.me)
	authed.POST("/auth/select-tenant", h.selectTenant)
}

func (h *Auth) setup(c *gin.Context) {
	var req service.SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		return
	}
	result, err := h.Svc.Setup(c.Request.Context(), req)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"token": result.Token,
		"me":    result.Me,
	}})
}

func (h *Auth) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		return
	}
	result, err := h.Svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"token": result.Token,
		"me":    result.Me,
	}})
}

func (h *Auth) selectTenant(c *gin.Context) {
	var req struct {
		TenantID int64 `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TenantID <= 0 {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "tenant_id is required")
		return
	}
	userID := userIDOf(c)
	result, err := h.Svc.SelectTenant(c.Request.Context(), userID, req.TenantID)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"token": result.Token,
		"me":    result.Me,
	}})
}

func (h *Auth) me(c *gin.Context) {
	view, unread, err := h.Svc.Me(c.Request.Context())
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"user":          view.User,
		"tenant":        view.Tenant,
		"tenants":       view.Tenants,
		"unread_count":  unread,
	}})
}

func (h *Auth) acceptInvitation(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "token and password are required")
		return
	}
	if err := h.Svc.AcceptInvitation(c.Request.Context(), req.Token, req.Password); err != nil {
		mapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
