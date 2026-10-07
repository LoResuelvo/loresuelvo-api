package installation_handler

import (
	"context"
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/gin-gonic/gin"
)

type Service interface {
	Register(context.Context, string, installation.Registration) (*installation.Installation, bool, error)
	Unregister(context.Context, string, string, string, string) error
}
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }
func (h *Handler) Register(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	var request registrationRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if err := c.ShouldBindJSON(&request); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid installation request")
		return
	}
	found, created, err := h.service.Register(c.Request.Context(), authID, installation.Registration{ID: c.Param("installation_id"), Secret: request.Secret, App: request.App, Token: request.Token, Locale: request.Locale, BindingID: request.BindingID, PreviousBindingID: request.PreviousBindingID})
	if err != nil {
		respondError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		c.Header("Location", "/installations/"+found.ID)
	}
	c.JSON(status, registrationResponse{ID: found.ID, BindingID: found.BindingID, App: found.App, Locale: found.Locale, Enabled: found.Enabled})
}
func (h *Handler) Unregister(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	var request removalRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if err := c.ShouldBindJSON(&request); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid installation request")
		return
	}
	if err := h.service.Unregister(c.Request.Context(), authID, c.Param("installation_id"), request.Secret, request.BindingID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, installation.ErrInvalidInstallation):
		httphandler.RespondError(c, http.StatusBadRequest, installation.ErrInvalidInstallation.Error())
	case errors.Is(err, installation.ErrForbidden):
		httphandler.RespondError(c, http.StatusForbidden, installation.ErrForbidden.Error())
	case errors.Is(err, installation.ErrConflict):
		httphandler.RespondError(c, http.StatusConflict, installation.ErrConflict.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "installation operation failed")
	}
}
