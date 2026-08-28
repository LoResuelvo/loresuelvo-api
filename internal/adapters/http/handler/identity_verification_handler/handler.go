package identity_verification_handler

import (
	"context"
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/gin-gonic/gin"
)

type identityVerificationStarter interface {
	Start(ctx context.Context, authID string) (identityverification.StartResult, error)
}

type IdentityVerificationHandler struct{ service identityVerificationStarter }

func NewIdentityVerificationHandler(service identityVerificationStarter) *IdentityVerificationHandler {
	return &IdentityVerificationHandler{service: service}
}

func (handler *IdentityVerificationHandler) StartSession(context *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(context)
	if !ok {
		return
	}
	result, err := handler.service.Start(context.Request.Context(), authID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := "identity verification could not be started"
		if errors.Is(err, identityverification.ErrProviderRequired) {
			statusCode = http.StatusForbidden
			message = "only registered providers can start identity verification"
		}
		if errors.Is(err, identityverification.ErrVerificationAlreadyApproved) {
			statusCode = http.StatusConflict
			message = "identity verification is already approved"
		}
		if errors.Is(err, identityverification.ErrVerifierUnavailable) {
			statusCode = http.StatusServiceUnavailable
			message = "identity verification is temporarily unavailable"
		}
		if errors.Is(err, identityverification.ErrVerifierMisconfigured) {
			statusCode = http.StatusBadGateway
			message = "identity verification could not be configured"
		}
		if errors.Is(err, identityverification.ErrVerificationAlreadyApproved) {
			statusCode = http.StatusConflict
			message = "identity verification is already approved"
		}
		httphandler.RespondError(context, statusCode, message)
		return
	}
	context.JSON(http.StatusOK, sessionResponse{
		SessionID: result.Credentials.SessionID, SessionToken: result.Credentials.SessionToken,
		VerificationURL: result.Credentials.VerificationURL, Status: result.Verification.Status,
	})
}
