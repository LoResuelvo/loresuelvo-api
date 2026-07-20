package mercadopago_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthClientBuildsAuthorizationCodeURLWithPKCE(t *testing.T) {
	client := mercadopago.NewOAuthClient(mercadopago.Config{
		ClientID:             "app-id",
		RedirectURI:          "https://api.loresuelvo.test/oauth/payment-accounts/callback",
		AuthorizationBaseURL: "https://auth.mercadopago.test/authorization",
	})

	authorizationURL, err := client.AuthorizationURL("state-secret", "pkce-verifier")

	require.NoError(t, err)
	parsedURL, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	assert.Equal(t, "app-id", parsedURL.Query().Get("client_id"))
	assert.Equal(t, "code", parsedURL.Query().Get("response_type"))
	assert.Equal(t, "mp", parsedURL.Query().Get("platform_id"))
	assert.Equal(t, "state-secret", parsedURL.Query().Get("state"))
	assert.Equal(t, "S-YYjGPeiHjsbIXpqrbVjcGUQn7X-4T468hBrBqm8pA", parsedURL.Query().Get("code_challenge"))
	assert.Equal(t, "S256", parsedURL.Query().Get("code_challenge_method"))
}

func TestConfigValidateRejectsMissingOAuthSettings(t *testing.T) {
	err := (mercadopago.Config{}).Validate()

	assert.ErrorIs(t, err, mercadopago.ErrInvalidOAuthConfiguration)
}

func TestNewOAuthClientFromEnvOwnsMercadoPagoOAuthConfiguration(t *testing.T) {
	t.Setenv("MERCADO_PAGO_CLIENT_ID", "app-id")
	t.Setenv("MERCADO_PAGO_CLIENT_SECRET", "app-secret")
	t.Setenv("MERCADO_PAGO_REDIRECT_URI", "https://api.loresuelvo.test/oauth/payment-accounts/callback")
	t.Setenv("MERCADO_PAGO_AUTHORIZATION_BASE_URL", "https://auth.mercadopago.test/authorization")
	t.Setenv("MERCADO_PAGO_API_BASE_URL", "https://api.mercadopago.test")

	client, err := mercadopago.NewOAuthClientFromEnv()

	require.NoError(t, err)
	authorizationURL, err := client.AuthorizationURL("state-secret", "pkce-challenge")
	require.NoError(t, err)
	parsedURL, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	assert.Equal(t, "app-id", parsedURL.Query().Get("client_id"))
	assert.Equal(t, "https://api.loresuelvo.test/oauth/payment-accounts/callback", parsedURL.Query().Get("redirect_uri"))
}

func TestOAuthClientExchangesAuthorizationCodeServerToServer(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/oauth/token", request.URL.Path)
		require.NoError(t, json.NewDecoder(request.Body).Decode(&received))
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"access_token":  "seller-access-token",
			"refresh_token": "seller-refresh-token",
			"user_id":       123456,
			"expires_in":    15552000,
		})
	}))
	t.Cleanup(server.Close)
	client := mercadopago.NewOAuthClient(mercadopago.Config{
		ClientID:     "app-id",
		ClientSecret: "app-secret",
		RedirectURI:  "https://api.loresuelvo.test/oauth/payment-accounts/callback",
		APIBaseURL:   server.URL,
	})
	beforeExchange := time.Now().UTC()

	credentials, err := client.ExchangeAuthorizationCode(context.Background(), "authorization-code", "pkce-verifier")

	require.NoError(t, err)
	assert.Equal(t, "app-id", received["client_id"])
	assert.Equal(t, "app-secret", received["client_secret"])
	assert.Equal(t, "authorization_code", received["grant_type"])
	assert.Equal(t, "authorization-code", received["code"])
	assert.Equal(t, "pkce-verifier", received["code_verifier"])
	assert.Equal(t, "123456", credentials.ExternalAccountID)
	assert.Equal(t, "seller-access-token", credentials.AccessToken)
	assert.Equal(t, "seller-refresh-token", credentials.RefreshToken)
	assert.True(t, credentials.CanReceiveMarketplacePayments)
	assert.WithinDuration(t, beforeExchange.Add(180*24*time.Hour), credentials.ExpiresOn, time.Second)
}
