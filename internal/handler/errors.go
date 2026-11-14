package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/service"
	"github.com/LinuxSuRen/tick-flow/internal/templating"
)

// mapError translates service/repo errors into the unified envelope. Every
// known error keeps a stable code; anything else is a 500 without internals.
func mapError(c *gin.Context, err error) {
	var ve *templating.ValidationError
	switch {
	case err == nil:
		return
	case errors.As(err, &ve):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, ve.Error())
	case isValidation(err):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
	case errors.Is(err, repository.ErrNotFound),
		errors.Is(err, service.ErrTicketNotFound),
		errors.Is(err, service.ErrAlreadySetup),
		errors.Is(err, service.ErrNotAMember),
		errors.Is(err, service.ErrInvalidInvite):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, service.ErrIllegalAction):
		httpx.Error(c, http.StatusConflict, httpx.CodeStateConflict, err.Error())
	case errors.Is(err, service.ErrRoleDenied),
		errors.Is(err, service.ErrInternalNoteForbidden):
		httpx.Error(c, http.StatusForbidden, httpx.CodeForbidden, err.Error())
	case errors.Is(err, repository.ErrAlreadyMember),
		errors.Is(err, repository.ErrSlugTaken):
		httpx.Error(c, http.StatusConflict, httpx.CodeDuplicated, err.Error())
	case errors.Is(err, service.ErrBadCredentials),
		errors.Is(err, service.ErrAccountNotActive):
		httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "email or password is incorrect")
	default:
		httpx.ErrInternal(c, err)
	}
}

// isValidation collects the plain validation sentinel errors.
func isValidation(err error) bool {
	var ev repository.ErrValidation
	if errors.As(err, &ev) {
		return true
	}
	switch {
	case errors.Is(err, service.ErrWeakPassword),
		errors.Is(err, service.ErrInvalidTenantSlug),
		errors.Is(err, service.ErrAttachmentTooLarge),
		errors.Is(err, service.ErrAttachmentType),
		errors.Is(err, service.ErrCommentTooLarge),
		errors.Is(err, service.ErrAssigneeInvalid):
		return true
	}
	return false
}
