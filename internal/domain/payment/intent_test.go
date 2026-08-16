package payment_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceBalanceIntentUsesWorkOrderRemainingAmounts(t *testing.T) {
	createdOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, createdOn)
	require.NoError(t, err)
	proposal := &serviceproposal.ServiceProposal{
		ID:           42,
		Consumer:     &consumer.Consumer{BaseUser: user.RehydrateBaseUser(10, "", "", "", "", consumer.Role, nil)},
		Provider:     &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "", "", "", "", provider.Role, nil)},
		BookingTerms: terms,
	}
	order := newWorkOrderFixture(t, 84, proposal, createdOn)

	intent, err := payment.NewServiceBalanceIntent(
		"83b4dd7d-6d1c-4e9e-b3e5-7be31b264540",
		order,
		createdOn,
	)

	require.NoError(t, err)
	assert.Equal(t, payment.PurposeServiceBalance, intent.Purpose)
	assert.Equal(t, proposal.ID, intent.ServiceProposalID)
	assert.Equal(t, terms.Currency(), intent.Currency)
	assert.Equal(t, terms.RemainingServiceBalanceCents(), intent.SellerAmountCents)
	assert.Equal(t, terms.RemainingPlatformFeeCents(), intent.PlatformFeeCents)
	assert.Equal(t, terms.RemainingAmountDueCents(), intent.TotalAmountCents)
	assert.Equal(t, payment.StatusRequiresCheckout, intent.Status)
	assert.Equal(t, createdOn, intent.CreatedOn)
	assert.Equal(t, createdOn, intent.UpdatedOn)
}

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

func TestBookingDepositIntentExpiresAtCheckoutDeadline(t *testing.T) {
	createdOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	expiresOn := createdOn.Add(30 * time.Minute)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, createdOn.Add(48*time.Hour))
	require.NoError(t, err)
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		terms,
		createdOn,
	)
	require.NoError(t, err)
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		expiresOn,
		createdOn.Add(time.Second),
	))

	assert.ErrorIs(t, intent.Expire(expiresOn.Add(-time.Nanosecond)), payment.ErrInvalidCheckoutSession)
	require.NoError(t, intent.Expire(expiresOn))
	assert.Equal(t, payment.StatusExpired, intent.Status)
	assert.Equal(t, expiresOn, intent.UpdatedOn)
}

func TestBookingDepositIntentProcessesPaymentIdempotentlyAndCanBecomePaid(t *testing.T) {
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
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		createdOn.Add(30*time.Minute),
		createdOn.Add(time.Second),
	))
	processingPayment := payment.ExternalPayment{
		ID:                "123456",
		SellerAccountID:   "mp-provider",
		ExternalReference: intent.ID,
		Status:            payment.ExternalPaymentStatusProcessing,
		Currency:          intent.Currency,
		AmountCents:       intent.TotalAmountCents,
	}

	require.NoError(t, intent.MarkProcessing(processingPayment, createdOn.Add(2*time.Second)))
	assert.Equal(t, payment.StatusProcessing, intent.Status)
	require.NoError(t, intent.MarkProcessing(processingPayment, createdOn.Add(3*time.Second)))
	assert.Equal(t, payment.StatusProcessing, intent.Status)

	approvedPayment := processingPayment
	approvedPayment.Status = payment.ExternalPaymentStatusApproved
	require.NoError(t, intent.MarkPaid(approvedPayment, createdOn.Add(4*time.Second)))
	assert.Equal(t, payment.StatusPaid, intent.Status)
}

func TestBookingDepositIntentCanRejectPaymentIdempotently(t *testing.T) {
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
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		createdOn.Add(30*time.Minute),
		createdOn.Add(time.Second),
	))
	rejectedPayment := payment.ExternalPayment{
		ID:                "123456",
		SellerAccountID:   "mp-provider",
		ExternalReference: intent.ID,
		Status:            payment.ExternalPaymentStatusRejected,
		Currency:          intent.Currency,
		AmountCents:       intent.TotalAmountCents,
	}

	require.NoError(t, intent.MarkRejected(rejectedPayment, createdOn.Add(2*time.Second)))
	assert.Equal(t, payment.StatusRejected, intent.Status)
	require.NoError(t, intent.MarkRejected(rejectedPayment, createdOn.Add(3*time.Second)))
	assert.Equal(t, payment.StatusRejected, intent.Status)
}
