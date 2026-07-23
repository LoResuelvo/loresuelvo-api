package repositories_test

import (
	"context"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentIntentRepositoryPersistsCheckoutReadyIntentAndSession(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		userRepository: testContext.userRepository,
	}, "auth0|payment-consumer", "payment.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|payment-provider", "payment.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	scheduledOn := now.Add(48 * time.Hour)
	proposal, err := serviceproposal.NewServiceProposal(
		&provider.Provider{BaseUser: user.RehydrateBaseUser(providerID, "", "", "", "", "", nil)},
		&consumer.Consumer{BaseUser: user.RehydrateBaseUser(consumerID, "", "", "", "", "", nil)},
		activeConversation,
		scheduledOn,
		"Reparacion de perdida de agua.",
		bookingTermsForAmount(t, 10000000, scheduledOn),
		clockadapter.NewSystemClock(),
	)
	require.NoError(t, err)
	proposal, err = testContext.serviceProposalRepository.Save(proposal)
	require.NoError(t, err)

	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		proposal.ID,
		proposal.BookingTerms,
		now,
	)
	require.NoError(t, err)
	repository := repositories.NewPaymentIntentRepository(testContext.database)
	require.NoError(t, repository.Save(context.Background(), intent))
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		now.Add(30*time.Minute),
		now.Add(time.Second),
	))

	err = repository.SaveCheckoutReady(context.Background(), intent)

	require.NoError(t, err)
	var storedStatus payment.IntentStatus
	var storedSellerAmount, storedPlatformFee, storedTotal int64
	require.NoError(t, testContext.database.QueryRow(
		`SELECT status, seller_amount_cents, platform_fee_cents, total_amount_cents
		FROM payment_intents
		WHERE id = $1`,
		intent.ID,
	).Scan(&storedStatus, &storedSellerAmount, &storedPlatformFee, &storedTotal))
	assert.Equal(t, payment.StatusCheckoutReady, storedStatus)
	assert.Equal(t, intent.SellerAmountCents, storedSellerAmount)
	assert.Equal(t, intent.PlatformFeeCents, storedPlatformFee)
	assert.Equal(t, intent.TotalAmountCents, storedTotal)

	var storedIntentID, storedPreferenceID, storedCheckoutURL string
	var storedExpiresOn time.Time
	require.NoError(t, testContext.database.QueryRow(
		`SELECT payment_intent_id, external_preference_id, checkout_url, expires_on
		FROM payment_checkout_sessions
		WHERE payment_intent_id = $1`,
		intent.ID,
	).Scan(&storedIntentID, &storedPreferenceID, &storedCheckoutURL, &storedExpiresOn))
	assert.Equal(t, intent.ID, storedIntentID)
	assert.Equal(t, intent.CheckoutSession.ExternalID, storedPreferenceID)
	assert.Equal(t, intent.CheckoutSession.URL, storedCheckoutURL)
	assert.True(t, intent.CheckoutSession.ExpiresOn.Equal(storedExpiresOn))
}
