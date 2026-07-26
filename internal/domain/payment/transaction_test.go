package payment_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPaymentTransactionKeepsVerifiedExternalPayment(t *testing.T) {
	verifiedOn := time.Date(2026, time.July, 4, 13, 5, 0, 0, time.UTC)
	externalPayment := payment.ExternalPayment{
		ID:                "123456",
		SellerAccountID:   "mp-provider",
		ExternalReference: "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		Status:            payment.ExternalPaymentStatusApproved,
		Currency:          "ARS",
		AmountCents:       2100000,
	}

	transaction, err := payment.NewTransaction(
		externalPayment.ExternalReference,
		paymentaccount.PaymentProvider("mercado_pago"),
		externalPayment,
		verifiedOn,
	)

	require.NoError(t, err)
	assert.Equal(t, externalPayment.ExternalReference, transaction.PaymentIntentID)
	assert.Equal(t, externalPayment.ID, transaction.ExternalPaymentID)
	assert.Equal(t, externalPayment.SellerAccountID, transaction.SellerAccountID)
	assert.Equal(t, externalPayment.Status, transaction.Status)
	assert.Equal(t, externalPayment.Currency, transaction.Currency)
	assert.Equal(t, externalPayment.AmountCents, transaction.AmountCents)
	assert.Equal(t, verifiedOn, transaction.VerifiedOn)
}

func TestNewPaymentTransactionRejectsMissingExternalPaymentIdentity(t *testing.T) {
	transaction, err := payment.NewTransaction(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		paymentaccount.PaymentProvider("mercado_pago"),
		payment.ExternalPayment{},
		time.Date(2026, time.July, 4, 13, 5, 0, 0, time.UTC),
	)

	assert.ErrorIs(t, err, payment.ErrInvalidPaymentTransaction)
	assert.Nil(t, transaction)
}
