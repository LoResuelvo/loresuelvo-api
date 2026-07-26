package payment

import (
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type VerifiedPayment interface {
	ExternalID() string
	SellerAccountID() string
	IntentID() string
	ApplyTo(intent *Intent, processor paymentaccount.PaymentProvider, now time.Time) (PaymentOutcome, error)
}

type PaymentOutcome interface {
	Accept(visitor PaymentOutcomeVisitor) error
}

type PaymentOutcomeVisitor interface {
	VisitIntentUpdated(IntentUpdated) error
	VisitBookingApproved(BookingApproved) error
}

type IntentUpdated struct {
	Intent *Intent
}

func (outcome IntentUpdated) Accept(visitor PaymentOutcomeVisitor) error {
	return visitor.VisitIntentUpdated(outcome)
}

type BookingApproved struct {
	Intent      *Intent
	Transaction *Transaction
}

func (outcome BookingApproved) Accept(visitor PaymentOutcomeVisitor) error {
	return visitor.VisitBookingApproved(outcome)
}

type processingPayment struct{ payment ExternalPayment }
type rejectedPayment struct{ payment ExternalPayment }
type approvedPayment struct{ payment ExternalPayment }

func NewVerifiedPayment(payment ExternalPayment) (VerifiedPayment, error) {
	switch payment.Status {
	case ExternalPaymentStatusProcessing:
		return processingPayment{payment: payment}, nil
	case ExternalPaymentStatusRejected:
		return rejectedPayment{payment: payment}, nil
	case ExternalPaymentStatusApproved:
		return approvedPayment{payment: payment}, nil
	default:
		return nil, ErrInvalidExternalPayment
	}
}

func (payment processingPayment) ExternalID() string      { return payment.payment.ID }
func (payment processingPayment) SellerAccountID() string { return payment.payment.SellerAccountID }
func (payment processingPayment) IntentID() string        { return payment.payment.ExternalReference }
func (payment rejectedPayment) ExternalID() string        { return payment.payment.ID }
func (payment rejectedPayment) SellerAccountID() string   { return payment.payment.SellerAccountID }
func (payment rejectedPayment) IntentID() string          { return payment.payment.ExternalReference }
func (payment approvedPayment) ExternalID() string        { return payment.payment.ID }
func (payment approvedPayment) SellerAccountID() string   { return payment.payment.SellerAccountID }
func (payment approvedPayment) IntentID() string          { return payment.payment.ExternalReference }

func (payment processingPayment) ApplyTo(
	intent *Intent,
	_ paymentaccount.PaymentProvider,
	now time.Time,
) (PaymentOutcome, error) {
	if err := intent.MarkProcessing(payment.payment, now); err != nil {
		return nil, err
	}
	return IntentUpdated{Intent: intent}, nil
}

func (payment rejectedPayment) ApplyTo(
	intent *Intent,
	_ paymentaccount.PaymentProvider,
	now time.Time,
) (PaymentOutcome, error) {
	if err := intent.MarkRejected(payment.payment, now); err != nil {
		return nil, err
	}
	return IntentUpdated{Intent: intent}, nil
}

func (payment approvedPayment) ApplyTo(
	intent *Intent,
	processor paymentaccount.PaymentProvider,
	now time.Time,
) (PaymentOutcome, error) {
	if err := intent.MarkPaid(payment.payment, now); err != nil {
		return nil, err
	}
	transaction, err := NewTransaction(intent.ID, processor, payment.payment, now)
	if err != nil {
		return nil, err
	}
	return BookingApproved{Intent: intent, Transaction: transaction}, nil
}
