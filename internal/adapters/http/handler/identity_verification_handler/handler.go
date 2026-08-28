package identity_verification_handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/gin-gonic/gin"
)

type identityVerificationStarter interface {
	Start(ctx context.Context, authID string) (identityverification.StartResult, error)
}

type identityVerificationResultApplier interface {
	ApplyResult(ctx context.Context, result identityverification.VerificationResult) error
}

type identityVerificationService interface {
	identityVerificationStarter
	identityVerificationResultApplier
}

type IdentityVerificationWebhook interface {
	Authenticate(body []byte, signature, timestamp string, now time.Time) error
	Translate(body []byte) (identityverification.VerificationResult, error)
}

type IdentityVerificationHandler struct {
	service       identityVerificationStarter
	resultApplier identityVerificationResultApplier
	webhook       IdentityVerificationWebhook
	clock         clock.Clock
}

func NewIdentityVerificationHandler(service identityVerificationStarter) *IdentityVerificationHandler {
	return &IdentityVerificationHandler{service: service}
}

func NewIdentityVerificationHandlerWithWebhook(
	service identityVerificationService,
	webhook IdentityVerificationWebhook,
	systemClock clock.Clock,
) *IdentityVerificationHandler {
	return &IdentityVerificationHandler{
		service:       service,
		resultApplier: service,
		webhook:       webhook,
		clock:         systemClock,
	}
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

const maxIdentityVerificationWebhookBodyBytes int64 = 2 * 1024 * 1024

func (handler *IdentityVerificationHandler) ProcessWebhook(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, maxIdentityVerificationWebhookBodyBytes)
	body, err := io.ReadAll(context.Request.Body)
	if err != nil {
		httphandler.RespondError(context, http.StatusBadRequest, "invalid identity verification webhook")
		return
	}

	if err := handler.webhook.Authenticate(
		body,
		context.GetHeader("X-Signature-V2"),
		context.GetHeader("X-Timestamp"),
		handler.clock.Now(),
	); err != nil {
		httphandler.RespondError(context, http.StatusUnauthorized, "invalid identity verification webhook signature")
		return
	}

	result, err := handler.webhook.Translate(body)
	if err != nil {
		httphandler.RespondError(context, http.StatusBadRequest, "invalid identity verification webhook")
		return
	}

	if err := handler.resultApplier.ApplyResult(context.Request.Context(), result); err != nil {
		handleIdentityVerificationWebhookError(context, err)
		return
	}
	context.Status(http.StatusNoContent)
}

func handleIdentityVerificationWebhookError(context *gin.Context, err error) {
	if errors.Is(err, identityverification.ErrInvalidVerification) {
		httphandler.RespondError(context, http.StatusBadRequest, "invalid identity verification webhook")
		return
	}
	if errors.Is(err, identityverification.ErrSessionNotFound) {
		httphandler.RespondError(context, http.StatusServiceUnavailable, "identity verification session is temporarily unavailable")
		return
	}
	httphandler.RespondError(context, http.StatusServiceUnavailable, "identity verification is temporarily unavailable")
}
