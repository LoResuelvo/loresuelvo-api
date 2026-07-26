package payment

import (
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

type BookingCheckoutPolicy struct{}

func (BookingCheckoutPolicy) Authorize(
	proposal *serviceproposal.ServiceProposal,
	consumerID int,
	now time.Time,
) error {
	if proposal == nil || proposal.Consumer == nil || proposal.Consumer.ID() != consumerID {
		return ErrOnlyProposalRecipientCanCheckout
	}
	if proposal.Status != serviceproposal.StatusPending {
		return ErrProposalNotPending
	}
	if proposal.Provider == nil {
		return ErrInvalidServiceProposal
	}
	if !now.UTC().Before(proposal.BookingTerms.BookingPaymentDeadline().UTC()) {
		return ErrBookingPaymentDeadlineReached
	}
	return nil
}

func (BookingCheckoutPolicy) Expiration(
	terms serviceproposal.BookingTerms,
	now time.Time,
	maximumValidity time.Duration,
) time.Time {
	expiresOn := now.UTC().Add(maximumValidity)
	deadline := terms.BookingPaymentDeadline().UTC()
	if deadline.Before(expiresOn) {
		return deadline
	}
	return expiresOn
}
