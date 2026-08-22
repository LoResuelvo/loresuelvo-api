package calendar_connection_handler

import (
	"context"
	"net/http"
	"strings"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/gin-gonic/gin"
)

type Service interface {
	StartAuthorization(ctx context.Context, authID string) (*calendarconnection.Authorization, error)
	CompleteAuthorization(ctx context.Context, state, code string) (*calendarconnection.Connection, error)
	RejectAuthorization(ctx context.Context, state string) error
}

type CalendarConnectionHandler struct {
	service              Service
	successRedirectURL   string
	cancelledRedirectURL string
}

func NewCalendarConnectionHandler(service Service, config Config) *CalendarConnectionHandler {
	return &CalendarConnectionHandler{
		service:              service,
		successRedirectURL:   config.ConnectionSuccessURL,
		cancelledRedirectURL: config.ConnectionCancelledURL,
	}
}

func (handler *CalendarConnectionHandler) StartAuthorization(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	authorization, err := handler.service.StartAuthorization(c.Request.Context(), authID)
	if err != nil {
		handleCalendarConnectionError(c, err)
		return
	}
	c.Header("Location", "/me")
	c.JSON(http.StatusCreated, authorizationResponse{
		AuthorizationURL: authorization.URL,
		State:            authorization.State,
	})
}

func (handler *CalendarConnectionHandler) CompleteAuthorization(c *gin.Context) {
	if providerError := strings.TrimSpace(c.Query("error")); providerError != "" {
		if providerError != "access_denied" {
			httphandler.RespondError(c, http.StatusBadRequest, "calendar authorization failed")
			return
		}
		if err := handler.service.RejectAuthorization(c.Request.Context(), c.Query("state")); err != nil {
			handleCalendarConnectionError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, handler.cancelledRedirectURL)
		return
	}

	if _, err := handler.service.CompleteAuthorization(c.Request.Context(), c.Query("state"), c.Query("code")); err != nil {
		handleCalendarConnectionError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, handler.successRedirectURL)
}
