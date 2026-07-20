package paymentaccount_test

import (
	"testing"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentAccountOwnsProtectedCredentialsAndExposesCapabilities(t *testing.T) {
	accessTokenCiphertext := []byte("encrypted-access-token")
	refreshTokenCiphertext := []byte("encrypted-refresh-token")
	expiresOn := time.Date(2027, time.January, 20, 12, 0, 0, 0, time.UTC)

	account, err := paymentaccount.NewPaymentAccount(
		42,
		paymentaccount.PaymentProvider("mercado_pago"),
		"  mp-juan  ",
		accessTokenCiphertext,
		refreshTokenCiphertext,
		expiresOn,
		true,
	)

	require.NoError(t, err)
	accessTokenCiphertext[0] = 'x'
	refreshTokenCiphertext[0] = 'x'
	returnedAccessTokenCiphertext := account.AccessTokenCiphertext()
	returnedAccessTokenCiphertext[0] = 'y'
	assert.Equal(t, 42, account.ProviderID())
	assert.Equal(t, paymentaccount.PaymentProvider("mercado_pago"), account.PaymentProvider())
	assert.Equal(t, "mp-juan", account.ExternalAccountID())
	assert.Equal(t, []byte("encrypted-access-token"), account.AccessTokenCiphertext())
	assert.Equal(t, []byte("encrypted-refresh-token"), account.RefreshTokenCiphertext())
	assert.Equal(t, expiresOn, account.TokenExpiresOn())
	assert.Equal(t, paymentaccount.StatusConnected, account.Status())
	assert.True(t, account.CanReceivePayments())
	assert.True(t, account.CanSendServiceProposals())
}
