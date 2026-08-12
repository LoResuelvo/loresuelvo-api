package mercadopago_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	sharedmercadopago "github.com/LoResuelvo/loresuelvo-api/internal/adapters/mercadopago"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthClientBuildsAuthorizationCodeURLWithPKCE(t *testing.T) {
	client, err := mercadopago.NewOAuthClient(mercadopago.Config{
		Environment:          sharedmercadopago.EnvironmentProduction,
		ClientID:             "app-id",
		ClientSecret:         "app-secret",
		RedirectURI:          "https://api.loresuelvo.test/oauth/payment-accounts/callback",
		AuthorizationBaseURL: "https://auth.mercadopago.test/authorization",
		APIBaseURL:           "https://api.mercadopago.test",
	})
	require.NoError(t, err)

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

func TestOAuthClientMapsUnusableAuthorizationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "invalid_grant"})
	}))
	t.Cleanup(server.Close)
	client, err := mercadopago.NewOAuthClient(mercadopago.Config{
		Environment: sharedmercadopago.EnvironmentProduction,
		ClientID:    "app-id", ClientSecret: "app-secret",
		RedirectURI:          "https://api.loresuelvo.test/oauth/payment-accounts/callback",
		AuthorizationBaseURL: "https://auth.mercadopago.test/authorization",
		APIBaseURL:           server.URL,
	})
	require.NoError(t, err)

	_, err = client.ExchangeAuthorizationCode(context.Background(), "expired-code", "pkce-verifier")

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationCodeUnusable)
}

func TestOAuthClientMapsUnavailableAuthorizationGrant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(response).Encode(map[string]string{"error": "unauthorized_client"})
	}))
	t.Cleanup(server.Close)
	client, err := mercadopago.NewOAuthClient(mercadopago.Config{
		Environment: sharedmercadopago.EnvironmentProduction,
		ClientID:    "app-id", ClientSecret: "app-secret",
		RedirectURI:          "https://api.loresuelvo.test/oauth/payment-accounts/callback",
		AuthorizationBaseURL: "https://auth.mercadopago.test/authorization",
		APIBaseURL:           server.URL,
	})
	require.NoError(t, err)

	_, err = client.ExchangeAuthorizationCode(context.Background(), "authorization-code", "pkce-verifier")

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationGrantUnavailable)
}

func TestConfigValidateRejectsMissingOAuthSettings(t *testing.T) {
	err := (mercadopago.Config{}).Validate()

	assert.ErrorIs(t, err, mercadopago.ErrInvalidOAuthConfiguration)
}

func TestNewOAuthClientFromEnvOwnsMercadoPagoOAuthConfiguration(t *testing.T) {
	t.Setenv("MERCADO_PAGO_CLIENT_ID", "app-id")
	t.Setenv("MERCADO_PAGO_CLIENT_SECRET", "app-secret")
	t.Setenv("MERCADO_PAGO_REDIRECT_URI", "https://api.loresuelvo.test/oauth/payment-accounts/callback")
	t.Setenv("MERCADO_PAGO_ENVIRONMENT", "sandbox")
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
	for _, testCase := range []struct {
		name        string
		environment sharedmercadopago.Environment
		testToken   bool
	}{
		{name: "sandbox", environment: sharedmercadopago.EnvironmentSandbox, testToken: true},
		{name: "production", environment: sharedmercadopago.EnvironmentProduction, testToken: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
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
					"scope":         "read write offline_access",
				})
			}))
			t.Cleanup(server.Close)
			client, err := mercadopago.NewOAuthClient(mercadopago.Config{
				Environment:          testCase.environment,
				ClientID:             "app-id",
				ClientSecret:         "app-secret",
				RedirectURI:          "https://api.loresuelvo.test/oauth/payment-accounts/callback",
				AuthorizationBaseURL: "https://auth.mercadopago.test/authorization",
				APIBaseURL:           server.URL,
			})
			require.NoError(t, err)
			beforeExchange := time.Now().UTC()

			credentials, err := client.ExchangeAuthorizationCode(context.Background(), "authorization-code", "pkce-verifier")

			require.NoError(t, err)
			assert.Equal(t, "app-id", received["client_id"])
			assert.Equal(t, "app-secret", received["client_secret"])
			assert.Equal(t, "authorization_code", received["grant_type"])
			assert.Equal(t, "authorization-code", received["code"])
			assert.Equal(t, "pkce-verifier", received["code_verifier"])
			assert.Equal(t, testCase.testToken, received["test_token"])
			assert.Equal(t, "123456", credentials.ExternalAccountID)
			assert.Equal(t, "seller-access-token", credentials.AccessToken)
			assert.Equal(t, "seller-refresh-token", credentials.RefreshToken)
			assert.WithinDuration(t, beforeExchange.Add(180*24*time.Hour), credentials.ExpiresOn, time.Second)
		})
	}
}

func TestNewOAuthClientFromEnvRejectsMissingOrInvalidEnvironment(t *testing.T) {
	t.Setenv("MERCADO_PAGO_CLIENT_ID", "app-id")
	t.Setenv("MERCADO_PAGO_CLIENT_SECRET", "app-secret")
	t.Setenv("MERCADO_PAGO_REDIRECT_URI", "https://api.loresuelvo.test/oauth/payment-accounts/callback")

	for _, environment := range []string{"", "test"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv("MERCADO_PAGO_ENVIRONMENT", environment)

			client, err := mercadopago.NewOAuthClientFromEnv()

			assert.Nil(t, client)
			assert.ErrorIs(t, err, mercadopago.ErrInvalidOAuthConfiguration)
		})
	}
}
