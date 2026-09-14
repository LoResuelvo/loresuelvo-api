package payment_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service              *payment.Service
	webhookVerifier      WebhookVerifier
	sellerAccountResolver ProviderAccountIDResolver
}

type WebhookVerifier interface {
	Validate(signature, requestID, dataID string) error
}

func NewPaymentHandler(service *payment.Service, webhookVerifier WebhookVerifier) *PaymentHandler {
	return &PaymentHandler{service: service, webhookVerifier: webhookVerifier}
}

// WithSellerAccountResolver wires the dependency that lets the payment demo
// simulator resolve the connected seller's Mercado Pago external account ID
// from a service proposal. When demo mode is disabled the resolver is never
// called, so leaving it nil is safe.
func (handler *PaymentHandler) WithSellerAccountResolver(resolver ProviderAccountIDResolver) *PaymentHandler {
	handler.sellerAccountResolver = resolver
	return handler
}

// ProviderAccountIDResolver returns the Mercado Pago external account ID
// associated with the connected seller of the given service proposal. The
// payment demo simulator needs this value to populate the demo ExternalPayment
// in a way that satisfies the verified-payment validation (matching
// seller account ID + correct intent reference + frozen pricing).
type ProviderAccountIDResolver interface {
	FindSellerAccountIDByProposalID(ctx context.Context, serviceProposalID int) (string, error)
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
	result, err := handler.service.StartBookingCheckout(context.Request.Context(), authID, proposalID)
	if err != nil {
		handleStartBookingCheckoutError(context, err)
		return
	}

	intent := result.Intent
	context.Header("Location", fmt.Sprintf("/payment-intents/%s", intent.ID))
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	context.JSON(status, checkoutSessionResponseFromDomain(intent))
}

func (handler *PaymentHandler) StartServiceBalanceCheckout(context *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(context)
	if !ok {
		return
	}
	workOrderID, err := httphandler.PositiveIDFromString(
		context.Param("workOrderID"),
		"work order id",
	)
	if err != nil {
		httphandler.RespondError(context, http.StatusBadRequest, err.Error())
		return
	}
	result, err := handler.service.StartServiceBalanceCheckout(
		context.Request.Context(),
		authID,
		workOrderID,
	)
	if err != nil {
		handleStartServiceBalanceCheckoutError(context, err)
		return
	}

	intent := result.Intent
	context.Header("Location", fmt.Sprintf("/payment-intents/%s", intent.ID))
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	context.JSON(status, serviceBalanceCheckoutResponseFromDomain(intent))
}

func (handler *PaymentHandler) GetIntent(context *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(context)
	if !ok {
		return
	}
	intent, err := handler.service.GetIntent(
		context.Request.Context(),
		authID,
		context.Param("paymentIntentID"),
	)
	if err != nil {
		switch {
		case errors.Is(err, payment.ErrIntentDoesNotExist):
			httphandler.RespondError(context, http.StatusNotFound, err.Error())
		case errors.Is(err, payment.ErrOnlyProposalRecipientCanView):
			httphandler.RespondError(context, http.StatusForbidden, err.Error())
		default:
			httphandler.RespondError(context, http.StatusInternalServerError, "Could not get payment intent")
		}
		return
	}
	if intent.Status == payment.StatusCheckoutReady && handler.demoModeCanSimulate(context, intent) {
		if err := handler.simulateDemoApproval(context, authID, intent); err != nil {
			httphandler.RespondError(context, http.StatusInternalServerError, "Could not simulate demo payment")
			return
		}
		intent, err = handler.service.GetIntent(
			context.Request.Context(),
			authID,
			intent.ID,
		)
		if err != nil {
			httphandler.RespondError(context, http.StatusInternalServerError, "Could not get payment intent")
			return
		}
	}
	context.JSON(http.StatusOK, paymentIntentResponseFromDomain(intent))
}

// demoModeCanSimulate reports whether the current request is allowed to
// trigger the demo simulator for the given intent. The service is the source
// of truth for "is demo mode enabled" (i.e. was the app started with
// PAYMENTS_DEMO_MODE=true in a permitted environment); the handler additionally
// guarantees the seller account lookup dependency is wired so we never panic
// with a nil pointer when demo mode is misconfigured.
func (handler *PaymentHandler) demoModeCanSimulate(
	context *gin.Context,
	intent *payment.Intent,
) bool {
	if !handler.service.IsDemoModeEnabled() {
		return false
	}
	if handler.sellerAccountResolver == nil {
		return false
	}
	return intent.ServiceProposalID > 0
}

func (handler *PaymentHandler) simulateDemoApproval(
	context *gin.Context,
	authID string,
	intent *payment.Intent,
) error {
	sellerAccountID, err := handler.sellerAccountResolver.FindSellerAccountIDByProposalID(
		context.Request.Context(),
		intent.ServiceProposalID,
	)
	if err != nil {
		return fmt.Errorf("resolving seller account for demo payment: %w", err)
	}
	return handler.service.SimulateApprovedDemoPayment(
		context.Request.Context(),
		intent,
		sellerAccountID,
	)
}

type mercadoPagoNotification struct {
	Type   string          `json:"type"`
	UserID json.RawMessage `json:"user_id"`
	Data   struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (handler *PaymentHandler) ProcessMercadoPagoWebhook(context *gin.Context) {
	dataID := context.Query("data.id")
	if handler.webhookVerifier == nil ||
		handler.webhookVerifier.Validate(
			context.GetHeader("X-Signature"),
			context.GetHeader("X-Request-Id"),
			dataID,
		) != nil {
		httphandler.RespondError(context, http.StatusUnauthorized, "Invalid webhook signature")
		return
	}
	var notification mercadoPagoNotification
	if err := context.ShouldBindJSON(&notification); err != nil ||
		notification.Type != "payment" ||
		notification.Data.ID == "" ||
		notification.Data.ID != dataID {
		httphandler.RespondError(context, http.StatusBadRequest, "Invalid payment notification")
		return
	}
	sellerAccountID, err := mercadoPagoUserID(notification.UserID)
	if err != nil {
		httphandler.RespondError(context, http.StatusBadRequest, "Invalid payment notification")
		return
	}
	if err := handler.service.ProcessPaymentNotification(
		context.Request.Context(),
		payment.PaymentNotification{
			ExternalPaymentID: notification.Data.ID,
			SellerAccountID:   sellerAccountID,
		},
	); err != nil {
		httphandler.RespondError(context, http.StatusInternalServerError, "Could not process payment notification")
		return
	}
	context.Status(http.StatusOK)
}

func mercadoPagoUserID(raw json.RawMessage) (string, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("Mercado Pago user id is required")
	}
	if strings.HasPrefix(value, `"`) {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil || strings.TrimSpace(decoded) == "" {
			return "", fmt.Errorf("Mercado Pago user id is invalid")
		}
		return strings.TrimSpace(decoded), nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("Mercado Pago user id is invalid")
	}
	return strconv.FormatInt(id, 10), nil
}

func handleStartBookingCheckoutError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, serviceproposal.ErrDoesNotExist):
		httphandler.RespondError(context, http.StatusNotFound, err.Error())
	case errors.Is(err, payment.ErrOnlyProposalRecipientCanCheckout):
		httphandler.RespondError(context, http.StatusForbidden, err.Error())
	case errors.Is(err, payment.ErrProposalNotPending),
		errors.Is(err, payment.ErrBookingPaymentDeadlineReached),
		errors.Is(err, paymentaccount.ErrConnectionNotFound):
		httphandler.RespondError(context, http.StatusConflict, err.Error())
	default:
		httphandler.RespondError(context, http.StatusInternalServerError, "Could not start booking checkout")
	}
}

func handleStartServiceBalanceCheckoutError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, workorder.ErrDoesNotExist):
		httphandler.RespondError(context, http.StatusNotFound, err.Error())
	case errors.Is(err, payment.ErrOnlyWorkOrderConsumerCanCheckout),
		errors.Is(err, workorder.ErrOnlyWorkOrderConsumerCanCheckout):
		httphandler.RespondError(context, http.StatusForbidden, err.Error())
	case errors.Is(err, payment.ErrWorkOrderAlreadyFullyPaid):
		httphandler.RespondError(context, http.StatusConflict, err.Error())
	case errors.Is(err, workorder.ErrWorkOrderAlreadyPaid):
		// Preserve the existing HTTP error contract while the domain owns the
		// transition decision.
		httphandler.RespondError(context, http.StatusConflict, payment.ErrWorkOrderAlreadyFullyPaid.Error())
	case errors.Is(err, payment.ErrWorkOrderNotScheduled),
		errors.Is(err, payment.ErrServiceBalancePaymentNotAvailable),
		errors.Is(err, workorder.ErrWorkOrderNotAwaitingPayment),
		errors.Is(err, workorder.ErrWorkOrderNotScheduledYet),
		errors.Is(err, paymentaccount.ErrConnectionNotFound):
		httphandler.RespondError(context, http.StatusConflict, err.Error())
	default:
		httphandler.RespondError(context, http.StatusInternalServerError, "Could not start service balance checkout")
	}
}
