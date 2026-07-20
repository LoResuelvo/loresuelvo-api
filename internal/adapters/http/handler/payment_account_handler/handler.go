package payment_account_handler

import (
	"net/http"
	"strings"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/gin-gonic/gin"
)

type PaymentAccountHandler struct {
	service              *paymentaccount.Service
	successRedirectURL   string
	cancelledRedirectURL string
}

func NewPaymentAccountHandler(service *paymentaccount.Service, config Config) *PaymentAccountHandler {
	return &PaymentAccountHandler{
		service:              service,
		successRedirectURL:   config.ConnectionSuccessURL,
		cancelledRedirectURL: config.ConnectionCancelledURL,
	}
}

func (h *PaymentAccountHandler) StartAuthorization(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	authorization, err := h.service.StartAuthorization(c.Request.Context(), authID)
	if err != nil {
		handlePaymentAccountError(c, err)
		return
	}

	c.Header("Location", "/providers/me/payment-accounts")
	c.JSON(http.StatusCreated, authorizationResponse{
		AuthorizationURL: authorization.URL,
		State:            authorization.State,
	})
}

func (h *PaymentAccountHandler) CompleteAuthorization(c *gin.Context) {
	providerError := strings.TrimSpace(c.Query("error"))
	if providerError != "" {
		if providerError != "access_denied" {
			httphandler.RespondError(c, http.StatusBadRequest, "payment account authorization failed")
			return
		}
		if err := h.service.RejectAuthorization(c.Request.Context(), c.Query("state")); err != nil {
			handlePaymentAccountError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, h.cancelledRedirectURL)
		return
	}

	_, err := h.service.CompleteAuthorization(
		c.Request.Context(),
		c.Query("state"),
		c.Query("code"),
	)
	if err != nil {
		handlePaymentAccountError(c, err)
		return
	}

	c.Redirect(http.StatusSeeOther, h.successRedirectURL)
}

func (h *PaymentAccountHandler) GetConnection(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	account, err := h.service.GetConnection(c.Request.Context(), authID)
	if err != nil {
		handlePaymentAccountError(c, err)
		return
	}
	c.JSON(http.StatusOK, connectionResponseFromDomain(account))
}
