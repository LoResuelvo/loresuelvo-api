package payment_account_handler_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigFromEnvOwnsFrontendRedirectConfiguration(t *testing.T) {
	t.Setenv(
		"PAYMENT_ACCOUNT_CONNECTION_SUCCESS_URL",
		"https://app.loresuelvo.test/provider/register/mercado-pago?result=success",
	)

	config, err := payment_account_handler.NewConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(
		t,
		"https://app.loresuelvo.test/provider/register/mercado-pago?result=success",
		config.ConnectionSuccessURL,
	)
}

func TestNewConfigFromEnvRejectsInvalidFrontendRedirectURL(t *testing.T) {
	t.Setenv("PAYMENT_ACCOUNT_CONNECTION_SUCCESS_URL", "not-a-url")

	_, err := payment_account_handler.NewConfigFromEnv()

	assert.Error(t, err)
}
