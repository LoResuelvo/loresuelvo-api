package payment

import (
	"strings"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type Transaction struct {
	ID                int
	PaymentIntentID   string
	Processor         paymentaccount.PaymentProvider
	ExternalPaymentID string
	SellerAccountID   string
	Status            ExternalPaymentStatus
	Currency          string
	AmountCents       int64
	VerifiedOn        time.Time
	CreatedOn         time.Time
	UpdatedOn         time.Time
}

func NewTransaction(
	paymentIntentID string,
	processor paymentaccount.PaymentProvider,
	externalPayment ExternalPayment,
	verifiedOn time.Time,
) (*Transaction, error) {
	validStatus := externalPayment.Status == ExternalPaymentStatusApproved ||
		externalPayment.Status == ExternalPaymentStatusProcessing ||
		externalPayment.Status == ExternalPaymentStatusRejected
	if strings.TrimSpace(paymentIntentID) == "" ||
		strings.TrimSpace(string(processor)) == "" ||
		strings.TrimSpace(externalPayment.ID) == "" ||
		strings.TrimSpace(externalPayment.SellerAccountID) == "" ||
		!validStatus ||
		strings.TrimSpace(externalPayment.Currency) == "" ||
		externalPayment.AmountCents <= 0 ||
		verifiedOn.IsZero() {
		return nil, ErrInvalidPaymentTransaction
	}

	verifiedOn = verifiedOn.UTC()
	return &Transaction{
		PaymentIntentID:   paymentIntentID,
		Processor:         processor,
		ExternalPaymentID: externalPayment.ID,
		SellerAccountID:   externalPayment.SellerAccountID,
		Status:            externalPayment.Status,
		Currency:          externalPayment.Currency,
		AmountCents:       externalPayment.AmountCents,
		VerifiedOn:        verifiedOn,
		CreatedOn:         verifiedOn,
		UpdatedOn:         verifiedOn,
	}, nil
}
