package payment_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookingDepositIntentBecomesCheckoutReady(t *testing.T) {
	createdOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, createdOn.Add(48*time.Hour))
	require.NoError(t, err)
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		terms,
		createdOn,
	)
	require.NoError(t, err)

	checkoutCreatedOn := createdOn.Add(time.Second)
	checkoutExpiresOn := createdOn.Add(30 * time.Minute)
	err = intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		checkoutExpiresOn,
		checkoutCreatedOn,
	)

	require.NoError(t, err)
	assert.Equal(t, payment.StatusCheckoutReady, intent.Status)
	require.NotNil(t, intent.CheckoutSession)
	assert.Equal(t, "mp-preference-42", intent.CheckoutSession.ExternalID)
	assert.Equal(t, checkoutExpiresOn, intent.CheckoutSession.ExpiresOn)
	assert.Equal(t, checkoutCreatedOn, intent.CheckoutSession.CreatedOn)
	assert.Equal(t, terms, intent.BookingTerms)
}

func TestBookingDepositIntentRejectsMissingBookingTerms(t *testing.T) {
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		serviceproposal.BookingTerms{},
		time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC),
	)

	assert.ErrorIs(t, err, payment.ErrInvalidIntent)
	assert.Nil(t, intent)
}

func TestBookingDepositIntentRejectsCheckoutExpirationAtCreationTime(t *testing.T) {
	createdOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, createdOn.Add(48*time.Hour))
	require.NoError(t, err)
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		terms,
		createdOn,
	)
	require.NoError(t, err)

	err = intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		createdOn,
		createdOn,
	)

	assert.ErrorIs(t, err, payment.ErrInvalidCheckoutSession)
	assert.Equal(t, payment.StatusRequiresCheckout, intent.Status)
	assert.Nil(t, intent.CheckoutSession)
}
