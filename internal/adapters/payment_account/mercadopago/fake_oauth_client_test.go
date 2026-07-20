package mercadopago_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeOAuthClientRepresentsMarketplaceDisabledAccount(t *testing.T) {
	credentials, err := mercadopago.NewFakeOAuthClient().ExchangeAuthorizationCode(
		context.Background(), "marketplace-disabled:mp-juan", "pkce-verifier",
	)

	require.NoError(t, err)
	assert.Equal(t, "mp-juan", credentials.ExternalAccountID)
	assert.False(t, credentials.CanReceiveMarketplacePayments)
}

func TestFakeOAuthClientRejectsUnusableAuthorizationCode(t *testing.T) {
	_, err := mercadopago.NewFakeOAuthClient().ExchangeAuthorizationCode(
		context.Background(), "expired", "pkce-verifier",
	)

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationCodeUnusable)
}
