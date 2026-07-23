package payment

import (
	"net/url"
	"strings"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

type Purpose string

const PurposeBookingDeposit Purpose = "booking_deposit"

type IntentStatus string

const (
	StatusRequiresCheckout IntentStatus = "requires_checkout"
	StatusCheckoutReady    IntentStatus = "checkout_ready"
)

type Intent struct {
	ID                string
	ServiceProposalID int
	Purpose           Purpose
	Currency          string
	SellerAmountCents int64
	PlatformFeeCents  int64
	TotalAmountCents  int64
	Status            IntentStatus
	CheckoutSession   *CheckoutSession
	BookingTerms      serviceproposal.BookingTerms
	CreatedOn         time.Time
	UpdatedOn         time.Time
}

type CheckoutSession struct {
	ExternalID string
	URL        string
	ExpiresOn  time.Time
	CreatedOn  time.Time
}

func NewBookingDepositIntent(
	id string,
	serviceProposalID int,
	bookingTerms serviceproposal.BookingTerms,
	now time.Time,
) (*Intent, error) {
	if strings.TrimSpace(id) == "" ||
		serviceProposalID <= 0 ||
		strings.TrimSpace(bookingTerms.Currency()) == "" ||
		bookingTerms.DepositCents() <= 0 ||
		bookingTerms.PlatformFeeDueNowCents() < 0 ||
		bookingTerms.AmountDueNowCents() != bookingTerms.DepositCents()+bookingTerms.PlatformFeeDueNowCents() ||
		now.IsZero() {
		return nil, ErrInvalidIntent
	}

	now = now.UTC()
	return &Intent{
		ID:                id,
		ServiceProposalID: serviceProposalID,
		Purpose:           PurposeBookingDeposit,
		Currency:          bookingTerms.Currency(),
		SellerAmountCents: bookingTerms.DepositCents(),
		PlatformFeeCents:  bookingTerms.PlatformFeeDueNowCents(),
		TotalAmountCents:  bookingTerms.AmountDueNowCents(),
		Status:            StatusRequiresCheckout,
		BookingTerms:      bookingTerms,
		CreatedOn:         now,
		UpdatedOn:         now,
	}, nil
}

func (intent *Intent) MarkCheckoutReady(externalID, checkoutURL string, expiresOn, now time.Time) error {
	parsedURL, err := url.ParseRequestURI(checkoutURL)
	if strings.TrimSpace(externalID) == "" ||
		err != nil ||
		parsedURL.Scheme != "https" ||
		parsedURL.Host == "" ||
		expiresOn.IsZero() ||
		now.IsZero() ||
		!expiresOn.After(now) ||
		intent.Status != StatusRequiresCheckout {
		return ErrInvalidCheckoutSession
	}

	now = now.UTC()
	intent.Status = StatusCheckoutReady
	intent.CheckoutSession = &CheckoutSession{
		ExternalID: externalID,
		URL:        checkoutURL,
		ExpiresOn:  expiresOn.UTC(),
		CreatedOn:  now,
	}
	intent.UpdatedOn = now
	return nil
}
