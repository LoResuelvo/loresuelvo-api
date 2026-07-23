package payment

import (
	"net/url"
	"strings"
	"time"
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
	CreatedOn         time.Time
	UpdatedOn         time.Time
}

type CheckoutSession struct {
	ExternalID string
	URL        string
	CreatedOn  time.Time
}

func NewBookingDepositIntent(
	id string,
	serviceProposalID int,
	currency string,
	sellerAmountCents int64,
	platformFeeCents int64,
	totalAmountCents int64,
	now time.Time,
) (*Intent, error) {
	if strings.TrimSpace(id) == "" ||
		serviceProposalID <= 0 ||
		strings.TrimSpace(currency) == "" ||
		sellerAmountCents <= 0 ||
		platformFeeCents < 0 ||
		totalAmountCents != sellerAmountCents+platformFeeCents ||
		now.IsZero() {
		return nil, ErrInvalidIntent
	}

	now = now.UTC()
	return &Intent{
		ID:                id,
		ServiceProposalID: serviceProposalID,
		Purpose:           PurposeBookingDeposit,
		Currency:          currency,
		SellerAmountCents: sellerAmountCents,
		PlatformFeeCents:  platformFeeCents,
		TotalAmountCents:  totalAmountCents,
		Status:            StatusRequiresCheckout,
		CreatedOn:         now,
		UpdatedOn:         now,
	}, nil
}

func (intent *Intent) MarkCheckoutReady(externalID, checkoutURL string, now time.Time) error {
	parsedURL, err := url.ParseRequestURI(checkoutURL)
	if strings.TrimSpace(externalID) == "" ||
		err != nil ||
		parsedURL.Scheme != "https" ||
		parsedURL.Host == "" ||
		now.IsZero() ||
		intent.Status != StatusRequiresCheckout {
		return ErrInvalidCheckoutSession
	}

	now = now.UTC()
	intent.Status = StatusCheckoutReady
	intent.CheckoutSession = &CheckoutSession{
		ExternalID: externalID,
		URL:        checkoutURL,
		CreatedOn:  now,
	}
	intent.UpdatedOn = now
	return nil
}
