package user_handler

import (
	"context"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/gin-gonic/gin"
)

type CalendarConnectionStatusReader interface {
	GetConnectionStatus(ctx context.Context, authID string) (string, error)
}

type IdentityVerificationStatusReader interface {
	CurrentStatus(ctx context.Context, providerID int) (identityverification.VerificationStatus, error)
}

type IdentityVerificationDetailsReader interface {
	CurrentStatusDetails(ctx context.Context, providerID int) (identityverification.VerificationStatusDetails, error)
}

type UserHandler struct {
	userService                      *user.Service
	calendarConnectionStatusReader   CalendarConnectionStatusReader
	identityVerificationStatusReader IdentityVerificationStatusReader
}

func NewUserHandler(userService *user.Service, statusReaders ...CalendarConnectionStatusReader) *UserHandler {
	var statusReader CalendarConnectionStatusReader
	if len(statusReaders) > 0 {
		statusReader = statusReaders[0]
	}
	return &UserHandler{
		userService:                    userService,
		calendarConnectionStatusReader: statusReader,
	}
}

func NewUserHandlerWithIdentityVerification(
	userService *user.Service,
	calendarConnectionStatusReader CalendarConnectionStatusReader,
	identityVerificationStatusReader IdentityVerificationStatusReader,
) *UserHandler {
	return &UserHandler{
		userService:                      userService,
		calendarConnectionStatusReader:   calendarConnectionStatusReader,
		identityVerificationStatusReader: identityVerificationStatusReader,
	}
}

func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	currentUser, err := h.userService.GetCurrentUser(c.Request.Context(), auth0ID)
	if err != nil {
		handleGetCurrentUserError(c, err)
		return
	}

	calendarStatus := calendarconnection.StatusDisconnected
	if h.calendarConnectionStatusReader != nil {
		calendarStatus, err = h.calendarConnectionStatusReader.GetConnectionStatus(c.Request.Context(), auth0ID)
		if err != nil {
			handleGetCurrentUserError(c, err)
			return
		}
	}
	response, err := currentUserResponseFromDomain(currentUser, calendarStatus)
	if err != nil {
		handleGetCurrentUserError(c, err)
		return
	}
	if currentProvider, ok := currentUser.(*provider.Provider); ok && h.identityVerificationStatusReader != nil {
		var details identityverification.VerificationStatusDetails
		if detailsReader, supportsDetails := h.identityVerificationStatusReader.(IdentityVerificationDetailsReader); supportsDetails {
			details, err = detailsReader.CurrentStatusDetails(c.Request.Context(), currentProvider.ID())
		} else {
			details.Status, err = h.identityVerificationStatusReader.CurrentStatus(c.Request.Context(), currentProvider.ID())
		}
		if err != nil {
			handleGetCurrentUserError(c, err)
			return
		}
		response, err = withIdentityVerificationDetails(response, details)
		if err != nil {
			handleGetCurrentUserError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, response)
}
