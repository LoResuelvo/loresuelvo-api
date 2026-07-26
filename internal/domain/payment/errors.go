package payment

import "errors"

var (
	ErrInvalidIntent                    = errors.New("Payment intent is invalid")
	ErrInvalidCheckoutSession           = errors.New("Checkout session is invalid")
	ErrInvalidExternalPayment           = errors.New("External payment does not match the payment intent")
	ErrOnlyProposalRecipientCanCheckout = errors.New("Only the service proposal recipient can start checkout")
	ErrOnlyProposalRecipientCanView     = errors.New("Only the service proposal recipient can view the payment intent")
	ErrProposalNotPending               = errors.New("Only pending service proposals can start checkout")
	ErrBookingPaymentDeadlineReached    = errors.New("Booking payment deadline has been reached")
	ErrIntentDoesNotExist               = errors.New("Payment intent does not exist")
	ErrInvalidPaymentTransaction        = errors.New("Payment transaction is invalid")
	ErrInvalidServiceProposal           = errors.New("Service proposal is invalid")
	ErrTransactionDoesNotExist          = errors.New("Payment transaction does not exist")
)
