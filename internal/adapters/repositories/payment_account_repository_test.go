package repositories_test

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type paymentAccountStoreTestContext struct {
	database     *sql.DB
	accountStore *repositories.PaymentAccountRepository
	attemptStore *repositories.AuthorizationAttemptRepository
	providerID   int
}

func newPaymentAccountRepositoryTest(t *testing.T) paymentAccountStoreTestContext {
	t.Helper()
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanPaymentAccountRepositoryTestDatabase(t, database)
		database.Close()
	})
	cleanPaymentAccountRepositoryTestDatabase(t, database)

	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           database,
		userRepository:     repositories.NewUserRepository(database),
		categoryRepository: repositories.NewCategoryRepository(database),
	}, "auth0|payment-account-provider", "payment.provider@example.com", "Juan", "Gomez", "Plomeria")

	authorizationAttemptRepository := repositories.NewAuthorizationAttemptRepository(database)
	return paymentAccountStoreTestContext{
		database:     database,
		accountStore: repositories.NewPaymentAccountRepository(database, authorizationAttemptRepository),
		attemptStore: authorizationAttemptRepository,
		providerID:   providerID,
	}
}

func cleanPaymentAccountRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"provider_payment_accounts",
		"payment_account_authorization_attempts",
		"providers",
		"users",
		"files",
		"categories",
	} {
		_, err := database.Exec("DELETE FROM " + table) // #nosec G201 -- fixed test-only table names.
		require.NoError(t, err, "could not clean %s", table)
	}
}

func TestPaymentAccountRepositoryCompletesAuthorizationAtomically(t *testing.T) {
	testContext := newPaymentAccountRepositoryTest(t)
	stateDigest := bytes.Repeat([]byte{3}, 32)
	attempt := &paymentaccount.AuthorizationAttempt{
		ProviderID:             testContext.providerID,
		PaymentProvider:        paymentaccount.PaymentProvider("mercado_pago"),
		StateDigest:            stateDigest,
		CodeVerifierCiphertext: []byte("opaque-pkce-ciphertext"),
		ExpiresOn:              time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, testContext.attemptStore.Save(context.Background(), attempt))
	tokenExpiresOn := time.Now().UTC().Add(180 * 24 * time.Hour).Truncate(time.Microsecond)
	account, err := paymentaccount.NewPaymentAccount(
		testContext.providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-juan",
		[]byte("opaque-access-token-ciphertext"),
		[]byte("opaque-refresh-token-ciphertext"),
		tokenExpiresOn,
		true,
	)
	require.NoError(t, err)

	err = testContext.accountStore.SaveFromAuthorization(context.Background(), attempt.ID, account)

	require.NoError(t, err)
	found, err := testContext.accountStore.FindByProviderID(context.Background(), testContext.providerID, paymentaccount.PaymentProvider("mercado_pago"))
	require.NoError(t, err)
	assert.Equal(t, account.ExternalAccountID(), found.ExternalAccountID())
	assert.Equal(t, account.AccessTokenCiphertext(), found.AccessTokenCiphertext())
	assert.Equal(t, account.RefreshTokenCiphertext(), found.RefreshTokenCiphertext())
	assert.Equal(t, account.TokenExpiresOn(), found.TokenExpiresOn())
	assert.True(t, found.CanReceivePayments())

	var accessTokenCiphertext []byte
	var consumedOn sql.NullTime
	require.NoError(t, testContext.database.QueryRow(
		"SELECT access_token_ciphertext FROM provider_payment_accounts WHERE provider_id = $1",
		testContext.providerID,
	).Scan(&accessTokenCiphertext))
	require.NoError(t, testContext.database.QueryRow(
		"SELECT consumed_on FROM payment_account_authorization_attempts WHERE id = $1",
		attempt.ID,
	).Scan(&consumedOn))
	assert.Equal(t, []byte("opaque-access-token-ciphertext"), accessTokenCiphertext)
	assert.True(t, consumedOn.Valid)

	secondAttempt := &paymentaccount.AuthorizationAttempt{
		ProviderID:             testContext.providerID,
		PaymentProvider:        paymentaccount.PaymentProvider("mercado_pago"),
		StateDigest:            bytes.Repeat([]byte{4}, 32),
		CodeVerifierCiphertext: []byte("second-opaque-pkce-ciphertext"),
		ExpiresOn:              time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, testContext.attemptStore.Save(context.Background(), secondAttempt))
	secondAccount, err := paymentaccount.NewPaymentAccount(
		testContext.providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-juan-secondary",
		[]byte("second-opaque-access-token-ciphertext"),
		nil,
		tokenExpiresOn,
		true,
	)
	require.NoError(t, err)

	err = testContext.accountStore.SaveFromAuthorization(context.Background(), secondAttempt.ID, secondAccount)

	require.ErrorIs(t, err, paymentaccount.ErrAlreadyConnected)
	require.NoError(t, testContext.database.QueryRow(
		"SELECT consumed_on FROM payment_account_authorization_attempts WHERE id = $1",
		secondAttempt.ID,
	).Scan(&consumedOn))
	assert.False(t, consumedOn.Valid)
}

func TestPaymentAccountRepositoryRollsBackAccountWhenAuthorizationAttemptCannotBeConsumed(t *testing.T) {
	testContext := newPaymentAccountRepositoryTest(t)
	account, err := paymentaccount.NewPaymentAccount(
		testContext.providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-juan",
		[]byte("opaque-access-token-ciphertext"),
		nil,
		time.Now().UTC().Add(180*24*time.Hour),
		true,
	)
	require.NoError(t, err)

	err = testContext.accountStore.SaveFromAuthorization(context.Background(), 999999, account)

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationAttemptNotFound)
	_, err = testContext.accountStore.FindByProviderID(
		context.Background(),
		testContext.providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
	)
	require.ErrorIs(t, err, paymentaccount.ErrConnectionNotFound)
}

func TestPaymentAccountRepositoryRejectsExternalAccountLinkedToAnotherProvider(t *testing.T) {
	testContext := newPaymentAccountRepositoryTest(t)
	secondProviderID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     repositories.NewUserRepository(testContext.database),
		categoryRepository: repositories.NewCategoryRepository(testContext.database),
	}, "auth0|second-payment-account-provider", "second.payment.provider@example.com", "Pedro", "Lopez", "Plomeria")
	provider := paymentaccount.PaymentProvider("mercado_pago")
	externalAccountID := "mp-shared-account"

	firstAttempt := &paymentaccount.AuthorizationAttempt{
		ProviderID:             testContext.providerID,
		PaymentProvider:        provider,
		StateDigest:            bytes.Repeat([]byte{5}, 32),
		CodeVerifierCiphertext: []byte("first-opaque-pkce-ciphertext"),
		ExpiresOn:              time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, testContext.attemptStore.Save(context.Background(), firstAttempt))
	firstAccount, err := paymentaccount.NewPaymentAccount(
		testContext.providerID, provider, externalAccountID,
		[]byte("first-access-token-ciphertext"), nil,
		time.Now().UTC().Add(180*24*time.Hour), true,
	)
	require.NoError(t, err)
	require.NoError(t, testContext.accountStore.SaveFromAuthorization(context.Background(), firstAttempt.ID, firstAccount))

	secondAttempt := &paymentaccount.AuthorizationAttempt{
		ProviderID:             secondProviderID,
		PaymentProvider:        provider,
		StateDigest:            bytes.Repeat([]byte{6}, 32),
		CodeVerifierCiphertext: []byte("second-opaque-pkce-ciphertext"),
		ExpiresOn:              time.Now().UTC().Add(10 * time.Minute),
	}
	require.NoError(t, testContext.attemptStore.Save(context.Background(), secondAttempt))
	secondAccount, err := paymentaccount.NewPaymentAccount(
		secondProviderID, provider, externalAccountID,
		[]byte("second-access-token-ciphertext"), nil,
		time.Now().UTC().Add(180*24*time.Hour), true,
	)
	require.NoError(t, err)

	err = testContext.accountStore.SaveFromAuthorization(context.Background(), secondAttempt.ID, secondAccount)

	require.ErrorIs(t, err, paymentaccount.ErrExternalAccountAlreadyLinked)
	_, err = testContext.accountStore.FindByProviderID(context.Background(), secondProviderID, provider)
	require.ErrorIs(t, err, paymentaccount.ErrConnectionNotFound)
	_, err = testContext.attemptStore.FindByStateDigest(context.Background(), secondAttempt.StateDigest)
	require.NoError(t, err, "rejected authorization attempt must remain unconsumed")
	foundFirst, err := testContext.accountStore.FindByProviderID(context.Background(), testContext.providerID, provider)
	require.NoError(t, err)
	assert.Equal(t, externalAccountID, foundFirst.ExternalAccountID())
}
