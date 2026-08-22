package calendar_connection_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/gin-gonic/gin"
)

func handleCalendarConnectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, calendarconnection.ErrAuthorizationStateRequired),
		errors.Is(err, calendarconnection.ErrAuthorizationCodeRequired),
		errors.Is(err, calendarconnection.ErrAuthorizationAttemptNotFound),
		errors.Is(err, calendarconnection.ErrAuthorizationAttemptExpired),
		errors.Is(err, calendarconnection.ErrAuthorizationAttemptConsumed),
		errors.Is(err, calendarconnection.ErrRefreshTokenRequired):
		httphandler.RespondError(c, http.StatusBadRequest, calendarConnectionDomainErrorMessage(err))
	case errors.Is(err, calendarconnection.ErrUserNotFound):
		httphandler.RespondError(c, http.StatusNotFound, calendarconnection.ErrUserNotFound.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "calendar connection operation failed")
	}
}

func calendarConnectionDomainErrorMessage(err error) string {
	for _, domainErr := range []error{
		calendarconnection.ErrAuthorizationStateRequired,
		calendarconnection.ErrAuthorizationCodeRequired,
		calendarconnection.ErrAuthorizationAttemptNotFound,
		calendarconnection.ErrAuthorizationAttemptExpired,
		calendarconnection.ErrAuthorizationAttemptConsumed,
		calendarconnection.ErrRefreshTokenRequired,
	} {
		if errors.Is(err, domainErr) {
			return domainErr.Error()
		}
	}
	return "calendar connection operation failed"
}
