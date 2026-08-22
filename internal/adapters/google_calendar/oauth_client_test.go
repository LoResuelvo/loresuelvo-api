package googlecalendar

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOAuthConfig(serverURL string) Config {
	return Config{
		ClientID:         "calendar-client-id",
		ClientSecret:     "calendar-client-secret",
		RedirectURI:      serverURL + "/oauth/google-calendar/callback",
		AuthorizationURL: serverURL + "/authorize",
		TokenURL:         serverURL + "/token",
	}
}

func TestOAuthClientBuildsAuthorizationURLWithOwnedEventsScopeAndPKCE(t *testing.T) {
	client, err := NewOAuthClient(testOAuthConfig("https://calendar.test"))
	require.NoError(t, err)

	authorizationURL, err := client.AuthorizationURL("state-value", "verifier-value")

	require.NoError(t, err)
	parsedURL, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	query := parsedURL.Query()
	digest := sha256.Sum256([]byte("verifier-value"))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(digest[:])
	assert.Equal(t, "calendar-client-id", query.Get("client_id"))
	assert.Equal(t, "code", query.Get("response_type"))
	assert.Equal(t, "state-value", query.Get("state"))
	assert.Equal(t, "https://calendar.test/oauth/google-calendar/callback", query.Get("redirect_uri"))
	assert.Equal(t, calendarEventsOwnedScope, query.Get("scope"))
	assert.Equal(t, "offline", query.Get("access_type"))
	assert.Equal(t, "consent", query.Get("prompt"))
	assert.Equal(t, expectedChallenge, query.Get("code_challenge"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
}

func TestOAuthClientExchangesAuthorizationCodeAndReturnsPrimaryCalendar(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", request.Header.Get("Content-Type"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		received, err = url.ParseQuery(string(body))
		require.NoError(t, err)
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"access_token":  "access-token-is-not-persisted",
			"refresh_token": "refresh-token",
		})
	}))
	defer server.Close()

	client, err := NewOAuthClient(testOAuthConfig(server.URL))
	require.NoError(t, err)
	client.httpClient = server.Client()

	credentials, err := client.ExchangeAuthorizationCode(context.Background(), "authorization-code", "verifier-value")

	require.NoError(t, err)
	assert.Equal(t, "primary", credentials.CalendarID)
	assert.Equal(t, "refresh-token", credentials.RefreshToken)
	assert.Equal(t, "calendar-client-id", received.Get("client_id"))
	assert.Equal(t, "calendar-client-secret", received.Get("client_secret"))
	assert.Equal(t, "authorization-code", received.Get("code"))
	assert.Equal(t, "verifier-value", received.Get("code_verifier"))
	assert.Equal(t, "authorization_code", received.Get("grant_type"))
	assert.Equal(t, "http://"+server.Listener.Addr().String()+"/oauth/google-calendar/callback", received.Get("redirect_uri"))
}

func TestOAuthClientMapsInvalidGoogleAuthorizationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, err := response.Write([]byte(`{"error":"invalid_grant"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewOAuthClient(testOAuthConfig(server.URL))
	require.NoError(t, err)
	client.httpClient = server.Client()

	_, err = client.ExchangeAuthorizationCode(context.Background(), "invalid-code", "verifier-value")

	assert.ErrorIs(t, err, ErrAuthorizationCodeUnusable)
}

func TestNewConfigFromEnvOwnsGoogleCalendarOAuthConfiguration(t *testing.T) {
	t.Setenv("GOOGLE_CALENDAR_CLIENT_ID", "env-client-id")
	t.Setenv("GOOGLE_CALENDAR_CLIENT_SECRET", "env-client-secret")
	t.Setenv("GOOGLE_CALENDAR_REDIRECT_URI", "https://app.test/oauth/google-calendar/callback")
	t.Setenv("GOOGLE_CALENDAR_AUTHORIZATION_URL", "https://accounts.test/authorize")
	t.Setenv("GOOGLE_CALENDAR_TOKEN_URL", "https://accounts.test/token")

	config, err := NewConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, "env-client-id", config.ClientID)
	assert.Equal(t, "env-client-secret", config.ClientSecret)
	assert.Equal(t, "https://app.test/oauth/google-calendar/callback", config.RedirectURI)
	assert.Equal(t, "https://accounts.test/authorize", config.AuthorizationURL)
	assert.Equal(t, "https://accounts.test/token", config.TokenURL)
}
