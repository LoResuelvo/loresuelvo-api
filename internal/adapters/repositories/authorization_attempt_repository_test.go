package repositories_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationAttemptRepositoryPersistsOpaqueAttempt(t *testing.T) {
	testContext := newPaymentAccountRepositoryTest(t)
	expiresOn := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Microsecond)
	stateDigest := bytes.Repeat([]byte{1}, 32)
	attempt := &paymentaccount.AuthorizationAttempt{
		ProviderID:             testContext.providerID,
		PaymentProvider:        paymentaccount.PaymentProvider("mercado_pago"),
		StateDigest:            stateDigest,
		CodeVerifierCiphertext: []byte("opaque-pkce-ciphertext"),
		ExpiresOn:              expiresOn,
	}

	err := testContext.attemptStore.Save(context.Background(), attempt)

	require.NoError(t, err)
	assert.Positive(t, attempt.ID)
	found, err := testContext.attemptStore.FindByStateDigest(context.Background(), stateDigest)
	require.NoError(t, err)
	assert.Equal(t, attempt.ID, found.ID)
	assert.Equal(t, testContext.providerID, found.ProviderID)
	assert.Equal(t, []byte("opaque-pkce-ciphertext"), found.CodeVerifierCiphertext)
	assert.Equal(t, expiresOn, found.ExpiresOn)
}

func TestAuthorizationAttemptRepositoryDoesNotHardcodePaymentProvider(t *testing.T) {
	testContext := newPaymentAccountRepositoryTest(t)
	stateDigest := bytes.Repeat([]byte{2}, 32)
	attempt := &paymentaccount.AuthorizationAttempt{
		ProviderID:             testContext.providerID,
		PaymentProvider:        paymentaccount.PaymentProvider("uala"),
		StateDigest:            stateDigest,
		CodeVerifierCiphertext: []byte("opaque-uala-verifier"),
		ExpiresOn:              time.Now().UTC().Add(10 * time.Minute),
	}

	err := testContext.attemptStore.Save(context.Background(), attempt)

	require.NoError(t, err)
	found, err := testContext.attemptStore.FindByStateDigest(context.Background(), stateDigest)
	require.NoError(t, err)
	assert.Equal(t, paymentaccount.PaymentProvider("uala"), found.PaymentProvider)
}
