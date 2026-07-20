package payment_account_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/gin-gonic/gin"
)

func handlePaymentAccountError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, paymentaccount.ErrOnlyProvidersCanConnect):
		httphandler.RespondError(c, http.StatusForbidden, paymentaccount.ErrOnlyProvidersCanConnect.Error())
	case errors.Is(err, paymentaccount.ErrAuthorizationStateRequired),
		errors.Is(err, paymentaccount.ErrAuthorizationCodeRequired),
		errors.Is(err, paymentaccount.ErrAuthorizationCodeUnusable),
		errors.Is(err, paymentaccount.ErrAuthorizationGrantUnavailable),
		errors.Is(err, paymentaccount.ErrAuthorizationAttemptExpired),
		errors.Is(err, paymentaccount.ErrAuthorizationAttemptNotFound),
		errors.Is(err, paymentaccount.ErrPaymentProviderMismatch):
		httphandler.RespondError(c, http.StatusBadRequest, paymentAccountDomainErrorMessage(err))
	case errors.Is(err, paymentaccount.ErrAlreadyConnected),
		errors.Is(err, paymentaccount.ErrExternalAccountAlreadyLinked):
		httphandler.RespondError(c, http.StatusConflict, paymentAccountDomainErrorMessage(err))
	case errors.Is(err, paymentaccount.ErrConnectionNotFound):
		c.JSON(http.StatusOK, connectionResponse{Status: "pending"})
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "payment account operation failed")
	}
}

func paymentAccountDomainErrorMessage(err error) string {
	for _, domainErr := range []error{
		paymentaccount.ErrAuthorizationStateRequired,
		paymentaccount.ErrAuthorizationCodeRequired,
		paymentaccount.ErrAuthorizationCodeUnusable,
		paymentaccount.ErrAuthorizationGrantUnavailable,
		paymentaccount.ErrAuthorizationAttemptExpired,
		paymentaccount.ErrAuthorizationAttemptNotFound,
		paymentaccount.ErrPaymentProviderMismatch,
		paymentaccount.ErrAlreadyConnected,
		paymentaccount.ErrExternalAccountAlreadyLinked,
	} {
		if errors.Is(err, domainErr) {
			return domainErr.Error()
		}
	}
	return "payment account operation failed"
}
