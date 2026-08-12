package paymentaccountadapter

import (
	"errors"
	"testing"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOAuthConnectorFromEnvRejectsUnsupportedPaymentProvider(t *testing.T) {
	t.Setenv("PAYMENT_ACCOUNT_PROVIDER", "uala")

	_, err := NewOAuthConnectorFromEnv()

	if !errors.Is(err, ErrUnsupportedPaymentProvider) {
		t.Fatalf("expected unsupported payment provider error, got %v", err)
	}
}

func TestNewOAuthConnectorFromEnvBuildsConfiguredOAuthConnector(t *testing.T) {
	t.Setenv("PAYMENT_ACCOUNT_PROVIDER", "mercado_pago")
	t.Setenv("MERCADO_PAGO_CLIENT_ID", "app-id")
	t.Setenv("MERCADO_PAGO_CLIENT_SECRET", "app-secret")
	t.Setenv("MERCADO_PAGO_REDIRECT_URI", "https://api.loresuelvo.test/oauth/payment-accounts/callback")
	t.Setenv("MERCADO_PAGO_ENVIRONMENT", "sandbox")

	oauthConnector, err := NewOAuthConnectorFromEnv()

	require.NoError(t, err)
	assert.Equal(t, paymentaccount.PaymentProvider("mercado_pago"), oauthConnector.Provider())
}
