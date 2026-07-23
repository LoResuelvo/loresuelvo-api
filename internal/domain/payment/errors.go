package payment

import "errors"

var (
	ErrInvalidIntent                    = errors.New("Payment intent is invalid")
	ErrInvalidCheckoutSession           = errors.New("Checkout session is invalid")
	ErrOnlyProposalRecipientCanCheckout = errors.New("Only the service proposal recipient can start checkout")
	ErrProposalNotPending               = errors.New("Only pending service proposals can start checkout")
)
