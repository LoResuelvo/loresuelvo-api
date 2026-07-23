package payment_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookingDepositIntentBecomesCheckoutReady(t *testing.T) {
	createdOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		"ARS",
		2000000,
		100000,
		2100000,
		createdOn,
	)
	require.NoError(t, err)

	checkoutCreatedOn := createdOn.Add(time.Second)
	err = intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		checkoutCreatedOn,
	)

	require.NoError(t, err)
	assert.Equal(t, payment.StatusCheckoutReady, intent.Status)
	require.NotNil(t, intent.CheckoutSession)
	assert.Equal(t, "mp-preference-42", intent.CheckoutSession.ExternalID)
	assert.Equal(t, checkoutCreatedOn, intent.CheckoutSession.CreatedOn)
}

func TestBookingDepositIntentRejectsInconsistentTotal(t *testing.T) {
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		"ARS",
		2000000,
		100000,
		2000000,
		time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC),
	)

	assert.ErrorIs(t, err, payment.ErrInvalidIntent)
	assert.Nil(t, intent)
}
