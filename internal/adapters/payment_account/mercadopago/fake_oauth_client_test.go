package mercadopago_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeOAuthClientReturnsAuthorizedSellerCredentials(t *testing.T) {
	credentials, err := mercadopago.NewFakeOAuthClient().ExchangeAuthorizationCode(
		context.Background(), "authorized:mp-juan", "pkce-verifier",
	)

	require.NoError(t, err)
	assert.Equal(t, "mp-juan", credentials.ExternalAccountID)
	assert.NotEmpty(t, credentials.AccessToken)
}

func TestFakeOAuthClientRejectsUnavailableAuthorizationGrant(t *testing.T) {
	_, err := mercadopago.NewFakeOAuthClient().ExchangeAuthorizationCode(
		context.Background(), "authorization-not-granted", "pkce-verifier",
	)

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationGrantUnavailable)
}

func TestFakeOAuthClientRejectsUnusableAuthorizationCode(t *testing.T) {
	_, err := mercadopago.NewFakeOAuthClient().ExchangeAuthorizationCode(
		context.Background(), "expired", "pkce-verifier",
	)

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationCodeUnusable)
}
