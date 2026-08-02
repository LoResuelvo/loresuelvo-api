package payment

import (
	"net/url"
	"strings"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type Purpose string

const (
	PurposeBookingDeposit Purpose = "booking_deposit"
	PurposeServiceBalance Purpose = "service_balance"
)

type IntentStatus string

const (
	StatusRequiresCheckout IntentStatus = "requires_checkout"
	StatusCheckoutReady    IntentStatus = "checkout_ready"
	StatusProcessing       IntentStatus = "processing"
	StatusPaid             IntentStatus = "paid"
	StatusRejected         IntentStatus = "rejected"
	StatusExpired          IntentStatus = "expired"
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

func NewServiceBalanceIntent(id string, order *workorder.WorkOrder, now time.Time) (*Intent, error) {
	if strings.TrimSpace(id) == "" ||
		order == nil ||
		order.ID <= 0 ||
		order.ServiceProposal == nil ||
		order.ServiceProposalID() <= 0 ||
		strings.TrimSpace(order.Currency()) == "" ||
		order.RemainingServiceBalance() <= 0 ||
		order.RemainingPlatformFee() < 0 ||
		order.RemainingAmountDue() != order.RemainingServiceBalance()+order.RemainingPlatformFee() ||
		now.IsZero() {
		return nil, ErrInvalidIntent
	}

	now = now.UTC()
	return &Intent{
		ID:                id,
		ServiceProposalID: order.ServiceProposalID(),
		Purpose:           PurposeServiceBalance,
		Currency:          order.Currency(),
		SellerAmountCents: order.RemainingServiceBalance(),
		PlatformFeeCents:  order.RemainingPlatformFee(),
		TotalAmountCents:  order.RemainingAmountDue(),
		Status:            StatusRequiresCheckout,
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

func (intent *Intent) MarkPaid(externalPayment ExternalPayment, now time.Time) error {
	if intent == nil ||
		(intent.Status != StatusCheckoutReady && intent.Status != StatusProcessing) ||
		strings.TrimSpace(externalPayment.ID) == "" ||
		externalPayment.Status != ExternalPaymentStatusApproved ||
		externalPayment.ExternalReference != intent.ID ||
		externalPayment.Currency != intent.Currency ||
		externalPayment.AmountCents != intent.TotalAmountCents ||
		now.IsZero() {
		return ErrInvalidExternalPayment
	}

	intent.Status = StatusPaid
	intent.UpdatedOn = now.UTC()
	return nil
}

func (intent *Intent) MarkProcessing(externalPayment ExternalPayment, now time.Time) error {
	if intent == nil ||
		(intent.Status != StatusCheckoutReady && intent.Status != StatusProcessing) ||
		strings.TrimSpace(externalPayment.ID) == "" ||
		externalPayment.Status != ExternalPaymentStatusProcessing ||
		externalPayment.ExternalReference != intent.ID ||
		externalPayment.Currency != intent.Currency ||
		externalPayment.AmountCents != intent.TotalAmountCents ||
		now.IsZero() {
		return ErrInvalidExternalPayment
	}

	intent.Status = StatusProcessing
	intent.UpdatedOn = now.UTC()
	return nil
}

func (intent *Intent) MarkRejected(externalPayment ExternalPayment, now time.Time) error {
	if intent == nil ||
		(intent.Status != StatusCheckoutReady &&
			intent.Status != StatusProcessing &&
			intent.Status != StatusRejected) ||
		strings.TrimSpace(externalPayment.ID) == "" ||
		externalPayment.Status != ExternalPaymentStatusRejected ||
		externalPayment.ExternalReference != intent.ID ||
		externalPayment.Currency != intent.Currency ||
		externalPayment.AmountCents != intent.TotalAmountCents ||
		now.IsZero() {
		return ErrInvalidExternalPayment
	}

	intent.Status = StatusRejected
	intent.UpdatedOn = now.UTC()
	return nil
}

func (intent *Intent) Expire(now time.Time) error {
	if intent == nil ||
		intent.Status != StatusCheckoutReady ||
		intent.CheckoutSession == nil ||
		now.IsZero() ||
		now.Before(intent.CheckoutSession.ExpiresOn) {
		return ErrInvalidCheckoutSession
	}

	intent.Status = StatusExpired
	intent.UpdatedOn = now.UTC()
	return nil
}

func (intent *Intent) PrepareCheckout(now time.Time) (bool, error) {
	if intent == nil {
		return false, nil
	}
	if intent.CheckoutSession == nil {
		if intent.Status == StatusRejected || intent.Status == StatusExpired {
			return false, nil
		}
		return false, ErrInvalidCheckoutSession
	}
	if intent.Status == StatusProcessing {
		return true, nil
	}
	if intent.Status != StatusCheckoutReady {
		return false, nil
	}
	if intent.CheckoutSession.ExpiresOn.After(now) {
		return true, nil
	}
	if err := intent.Expire(now); err != nil {
		return false, err
	}
	return false, nil
}
