package googlecalendar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
)

type OAuthClient struct {
	config     Config
	httpClient *http.Client
}

func NewOAuthClient(config Config) (*OAuthClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &OAuthClient{
		config:     config,
		httpClient: observability.NewLoggingHTTPClient("google_calendar", "exchange_authorization_code", 10*time.Second),
	}, nil
}

func NewOAuthClientFromEnv() (*OAuthClient, error) {
	config, err := NewConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewOAuthClient(config)
}

func (client *OAuthClient) AuthorizationURL(state, codeVerifier string) (string, error) {
	if client == nil || client.config.ClientID == "" || client.config.RedirectURI == "" || client.config.AuthorizationURL == "" {
		return "", ErrInvalidOAuthConfiguration
	}
	authorizationURL, err := url.Parse(client.config.AuthorizationURL)
	if err != nil {
		return "", fmt.Errorf("parsing Google Calendar authorization URL: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("client_id", client.config.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", client.config.RedirectURI)
	query.Set("scope", calendarEventsOwnedScope)
	query.Set("state", state)
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	return authorizationURL.String(), nil
}

func pkceChallenge(codeVerifier string) string {
	digest := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (client *OAuthClient) ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (calendarconnection.AuthorizationCredentials, error) {
	if client == nil || client.config.ClientID == "" || client.config.ClientSecret == "" || client.config.RedirectURI == "" || client.config.TokenURL == "" {
		return calendarconnection.AuthorizationCredentials{}, ErrInvalidOAuthConfiguration
	}
	form := url.Values{
		"client_id":     []string{client.config.ClientID},
		"client_secret": []string{client.config.ClientSecret},
		"code":          []string{code},
		"code_verifier": []string{codeVerifier},
		"grant_type":    []string{"authorization_code"},
		"redirect_uri":  []string{client.config.RedirectURI},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.TokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return calendarconnection.AuthorizationCredentials{}, fmt.Errorf("creating Google Calendar token request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpClient := client.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return calendarconnection.AuthorizationCredentials{}, fmt.Errorf("requesting Google Calendar token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return calendarconnection.AuthorizationCredentials{}, mapTokenError(response.Body, response.StatusCode)
	}
	var tokenResponse struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
		return calendarconnection.AuthorizationCredentials{}, fmt.Errorf("decoding Google Calendar token response: %w", err)
	}
	return calendarconnection.AuthorizationCredentials{
		CalendarID:   primaryCalendarID,
		RefreshToken: tokenResponse.RefreshToken,
	}, nil
}

func mapTokenError(body io.Reader, statusCode int) error {
	var oauthError struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(body).Decode(&oauthError); err == nil {
		switch oauthError.Error {
		case "invalid_grant":
			return ErrAuthorizationCodeUnusable
		case "unauthorized_client":
			return ErrAuthorizationGrantUnavailable
		}
	}
	return fmt.Errorf("Google Calendar token request returned status %d", statusCode)
}

var _ calendarconnection.OAuthConnector = (*OAuthClient)(nil)
