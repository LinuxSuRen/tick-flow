package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/middleware"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Tenant exposes the current tenant and branding updates (issue #11).
type Tenant struct {
	Svc *service.TenantService
}

// Register mounts tenant routes; PATCH requires tenant_admin.
func (h *Tenant) Register(rg *gin.RouterGroup) {
	rg.GET("/tenants/current", h.current)
	rg.PATCH("/tenants/current", middleware.RequireRole(model.RoleTenantAdmin), h.patch)
}

func (h *Tenant) current(c *gin.Context) {
	data, err := h.Svc.CurrentTenant(c.Request.Context())
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Tenant) patch(c *gin.Context) {
	var req struct {
		Name    string          `json:"name"`
		LogoURL string          `json:"logo_url"`
		Theme   json.RawMessage `json:"theme"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		return
	}
	data, err := h.Svc.UpdateBranding(c.Request.Context(), req.Name, req.LogoURL, req.Theme)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
