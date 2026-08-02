package payment_handler

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

type checkoutSessionResponse struct {
	PaymentIntentID string               `json:"payment_intent_id"`
	Status          string               `json:"status"`
	CheckoutURL     string               `json:"checkout_url"`
	ExpiresOn       time.Time            `json:"expires_on"`
	Pricing         bookingTermsResponse `json:"pricing"`
}

type bookingTermsResponse struct {
	Currency                     string    `json:"currency"`
	ServiceTotalCents            int64     `json:"service_total_cents"`
	DepositCents                 int64     `json:"deposit_cents"`
	RemainingServiceBalanceCents int64     `json:"remaining_service_balance_cents"`
	PlatformFeeTotalCents        int64     `json:"platform_fee_total_cents"`
	PlatformFeeDueNowCents       int64     `json:"platform_fee_due_now_cents"`
	RemainingPlatformFeeCents    int64     `json:"remaining_platform_fee_cents"`
	AmountDueNowCents            int64     `json:"amount_due_now_cents"`
	RemainingAmountDueCents      int64     `json:"remaining_amount_due_cents"`
	ContractTotalCents           int64     `json:"contract_total_cents"`
	BookingPaymentDeadline       time.Time `json:"booking_payment_deadline"`
}

type paymentIntentResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type serviceBalanceCheckoutResponse struct {
	PaymentIntentID string                        `json:"payment_intent_id"`
	Status          string                        `json:"status"`
	CheckoutURL     string                        `json:"checkout_url"`
	ExpiresOn       time.Time                     `json:"expires_on"`
	Pricing         serviceBalancePricingResponse `json:"pricing"`
}

type serviceBalancePricingResponse struct {
	Currency                     string `json:"currency"`
	RemainingServiceBalanceCents int64  `json:"remaining_service_balance_cents"`
	RemainingPlatformFeeCents    int64  `json:"remaining_platform_fee_cents"`
	AmountDueNowCents            int64  `json:"amount_due_now_cents"`
}

func paymentIntentResponseFromDomain(intent *payment.Intent) paymentIntentResponse {
	return paymentIntentResponse{
		ID:     intent.ID,
		Status: string(intent.Status),
	}
}

func checkoutSessionResponseFromDomain(intent *payment.Intent) checkoutSessionResponse {
	return checkoutSessionResponse{
		PaymentIntentID: intent.ID,
		Status:          string(intent.Status),
		CheckoutURL:     intent.CheckoutSession.URL,
		ExpiresOn:       intent.CheckoutSession.ExpiresOn,
		Pricing:         bookingTermsResponseFromDomain(intent.BookingTerms),
	}
}

func serviceBalanceCheckoutResponseFromDomain(intent *payment.Intent) serviceBalanceCheckoutResponse {
	return serviceBalanceCheckoutResponse{
		PaymentIntentID: intent.ID,
		Status:          string(intent.Status),
		CheckoutURL:     intent.CheckoutSession.URL,
		ExpiresOn:       intent.CheckoutSession.ExpiresOn,
		Pricing: serviceBalancePricingResponse{
			Currency:                     intent.Currency,
			RemainingServiceBalanceCents: intent.SellerAmountCents,
			RemainingPlatformFeeCents:    intent.PlatformFeeCents,
			AmountDueNowCents:            intent.TotalAmountCents,
		},
	}
}

func bookingTermsResponseFromDomain(terms serviceproposal.BookingTerms) bookingTermsResponse {
	return bookingTermsResponse{
		Currency:                     terms.Currency(),
		ServiceTotalCents:            terms.ServiceTotalCents(),
		DepositCents:                 terms.DepositCents(),
		RemainingServiceBalanceCents: terms.RemainingServiceBalanceCents(),
		PlatformFeeTotalCents:        terms.PlatformFeeTotalCents(),
		PlatformFeeDueNowCents:       terms.PlatformFeeDueNowCents(),
		RemainingPlatformFeeCents:    terms.RemainingPlatformFeeCents(),
		AmountDueNowCents:            terms.AmountDueNowCents(),
		RemainingAmountDueCents:      terms.RemainingAmountDueCents(),
		ContractTotalCents:           terms.ContractTotalCents(),
		BookingPaymentDeadline:       terms.BookingPaymentDeadline(),
	}
}
