// Package middleware provides Gin middlewares: observability (request id,
// access log, panic recovery) and authentication (JWT parsing, role checks).
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/httpx"
)

// requestIDKey is the gin context key holding the request id.
const requestIDKey = "request_id"

// headerRequestID is the inbound/outbound request id header.
const headerRequestID = "X-Request-ID"

// RequestID assigns a request id: an inbound X-Request-ID header when present,
// otherwise a freshly generated random id. The id lands on the response
// header and the gin context for structured logging.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(headerRequestID)
		if id == "" {
			buf := make([]byte, 8)
			_, _ = rand.Read(buf)
			id = hex.EncodeToString(buf)
		}
		c.Set(requestIDKey, id)
		c.Header(headerRequestID, id)
		c.Next()
	}
}

// AccessLog emits one structured log line per request. Health probes are
// logged at debug level to keep the default output clean.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", GetRequestID(c),
			"client_ip", c.ClientIP(),
		}
		if c.Request.URL.Path == "/healthz" {
			slog.Debug("http access", attrs...)
			return
		}
		slog.Info("http access", attrs...)
	}
}

// GetRequestID returns the request id assigned by RequestID.
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Recovery converts panics into the unified error envelope. The stack is
// logged server side and never leaked to the client.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"request_id", GetRequestID(c),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"panic", err,
					"stack", stackTrace(),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    httpx.CodeInternalError,
						"message": "internal server error",
					},
				})
			}
		}()
		c.Next()
	}
}

// RequireAuth verifies the Bearer JWT and injects the authenticated context
// (user id, tenant id, role). The tenant identity comes exclusively from the
// token claims; no tenant parameter is ever read from the request.
func RequireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := parseBearerToken(c.GetHeader("Authorization"), secret)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}
		ac := &appctx.AuthContext{
			UserID:   claims.UserID,
			TenantID: claims.Tenant,
			Role:     claims.Role,
		}
		c.Request = c.Request.WithContext(appctx.With(c.Request.Context(), ac))
		c.Next()
	}
}

// RequireRole restricts the route to the given tenant roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		ac, ok := appctx.From(c.Request.Context())
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing authentication")
			c.Abort()
			return
		}
		if _, ok := allowed[ac.Role]; !ok {
			httpx.Error(c, http.StatusForbidden, httpx.CodeForbidden, "role not permitted")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireTenant rejects tokens that carry no tenant binding (issued by
// login before tenant selection).
func RequireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		ac, ok := appctx.From(c.Request.Context())
		if !ok || ac.TenantID == 0 {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "no tenant selected")
			c.Abort()
			return
		}
		c.Next()
	}
}
