// Package httpx centralizes the HTTP response envelope: the unified error
// structure and the pagination contract shared by every JSON endpoint.
package httpx

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Stable error codes. The frontend maps these to user-facing copy; the
// message returned alongside is developer-facing only.
const (
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeValidationFailed = "VALIDATION_FAILED"
	CodeStateConflict   = "STATE_CONFLICT"
	CodeDuplicated      = "DUPLICATED"
	CodeRateLimited     = "RATE_LIMITED"
	CodeInternalError   = "INTERNAL_ERROR"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error writes a non-2xx response in the unified error envelope.
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}

// ErrInternal writes a generic 500 without leaking internals. The cause is
// logged server side with the request id when available.
func ErrInternal(c *gin.Context, err error) {
	if err != nil {
		c.Error(err)
		attrs := []any{"error", err}
		if rid := c.GetHeader("X-Request-ID"); rid != "" {
			attrs = append(attrs, "request_id", rid)
		}
		slog.Error("internal error", attrs...)
	}
	Error(c, http.StatusInternalServerError, CodeInternalError, "internal server error")
}

// PageData is the unified paginated response body.
type PageData struct {
	Data     any   `json:"data"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// Page writes a successful paginated response.
func Page(c *gin.Context, data any, page, pageSize int, total int64) {
	c.JSON(http.StatusOK, PageData{Data: data, Page: page, PageSize: pageSize, Total: total})
}

// Pagination bounds per the API contract: page starts at 1, page_size defaults
// to 20 and is capped at 100.
func NormalizePagination(page, pageSize int) (int, int) {
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

// IsNoRows reports whether err means "not found" for row scans.
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
