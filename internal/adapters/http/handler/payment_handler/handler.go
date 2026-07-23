package payment_handler

import (
	"errors"
	"fmt"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service *payment.Service
}

func NewPaymentHandler(service *payment.Service) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (handler *PaymentHandler) StartBookingCheckout(context *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(context)
	if !ok {
		return
	}
	proposalID, err := httphandler.PositiveIDFromString(
		context.Param("serviceProposalID"),
		"service proposal id",
	)
	if err != nil {
		httphandler.RespondError(context, http.StatusBadRequest, err.Error())
		return
	}
	intent, err := handler.service.StartBookingCheckout(context.Request.Context(), authID, proposalID)
	if err != nil {
		handleStartBookingCheckoutError(context, err)
		return
	}

	context.Header("Location", fmt.Sprintf("/payment-intents/%s", intent.ID))
	context.JSON(http.StatusCreated, checkoutSessionResponseFromDomain(intent))
}

func handleStartBookingCheckoutError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, serviceproposal.ErrDoesNotExist):
		httphandler.RespondError(context, http.StatusNotFound, err.Error())
	case errors.Is(err, payment.ErrOnlyProposalRecipientCanCheckout):
		httphandler.RespondError(context, http.StatusForbidden, err.Error())
	case errors.Is(err, payment.ErrProposalNotPending),
		errors.Is(err, paymentaccount.ErrConnectionNotFound):
		httphandler.RespondError(context, http.StatusConflict, err.Error())
	default:
		httphandler.RespondError(context, http.StatusInternalServerError, "Could not start booking checkout")
	}
}
