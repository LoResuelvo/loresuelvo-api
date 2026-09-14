package payment

import (
	"errors"
	"fmt"
	"time"
)

// Demo payment identifier prefix. Demo-mode payment IDs are deterministically
// derived from the PaymentIntent ID so they are stable across the polling loop
// and cannot be confused with real Mercado Pago payment IDs (those are integer
// numerics; these always start with the explicit prefix below).
const demoExternalPaymentIDPrefix = "demo-payment-"

// ErrDemoPaymentDisabled is returned by SimulateApprovedDemoPayment when the
// service was constructed without the demo-mode flag enabled.
var ErrDemoPaymentDisabled = errors.New("payment demo mode is disabled")

// ErrDemoPaymentIntentNotReady is returned by SimulateApprovedDemoPayment when
// the target intent is not in a state that can be approved (e.g. already paid,
// expired, or never reached the checkout_ready status).
var ErrDemoPaymentIntentNotReady = errors.New("payment intent is not ready for demo approval")

// demoExternalPaymentID returns the deterministic external payment ID used by
// demo-mode simulations for a given intent.
func demoExternalPaymentID(intentID string) string {
	return fmt.Sprintf("%s%s", demoExternalPaymentIDPrefix, intentID)
}

// buildDemoExternalPayment builds the ExternalPayment that mirrors what the real
// Mercado Pago webhook would carry for an approved payment of the given
// intent. The values are sourced from the intent itself so the validation
// performed by Intent.MarkPaid and Transaction.NewTransaction succeed.
func buildDemoExternalPayment(intent *Intent, sellerAccountID string, now time.Time) ExternalPayment {
	return ExternalPayment{
		ID:                demoExternalPaymentID(intent.ID),
		SellerAccountID:   sellerAccountID,
		ExternalReference: intent.ID,
		Status:            ExternalPaymentStatusApproved,
		Currency:          intent.Currency,
		AmountCents:       intent.TotalAmountCents,
	}
}
