package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
)

// userIDOf extracts the authenticated user id from the request context.
func userIDOf(c *gin.Context) int64 {
	ac, _ := appctx.From(c.Request.Context())
	if ac != nil {
		return ac.UserID
	}
	return 0
}
