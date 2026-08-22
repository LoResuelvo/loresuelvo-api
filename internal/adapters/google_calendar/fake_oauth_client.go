package googlecalendar

import (
	"context"
	"net/url"
	"strings"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
)

type FakeOAuthClient struct{}

func NewFakeOAuthClient() *FakeOAuthClient {
	return &FakeOAuthClient{}
}

func (client *FakeOAuthClient) AuthorizationURL(state, codeVerifier string) (string, error) {
	authorizationURL := &url.URL{
		Scheme: "https",
		Host:   "accounts.google.test",
		Path:   "/o/oauth2/v2/auth",
	}
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("redirect_uri", "http://localhost:8080/oauth/google-calendar/callback")
	query.Set("scope", calendarEventsOwnedScope)
	query.Set("state", state)
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	return authorizationURL.String(), nil
}

func (client *FakeOAuthClient) ExchangeAuthorizationCode(_ context.Context, code, _ string) (calendarconnection.AuthorizationCredentials, error) {
	const authorizedPrefix = "authorized:"
	if code == "android-calendar-code" {
		return calendarconnection.AuthorizationCredentials{
			CalendarID:   primaryCalendarID,
			RefreshToken: "fake-refresh-token-for-android",
		}, nil
	}
	if !strings.HasPrefix(code, authorizedPrefix) {
		return calendarconnection.AuthorizationCredentials{}, ErrAuthorizationCodeUnusable
	}
	accountID := strings.TrimPrefix(code, authorizedPrefix)
	if strings.TrimSpace(accountID) == "" {
		return calendarconnection.AuthorizationCredentials{}, ErrAuthorizationCodeUnusable
	}
	return calendarconnection.AuthorizationCredentials{
		CalendarID:   primaryCalendarID,
		RefreshToken: "fake-refresh-token-for-" + accountID,
	}, nil
}

var _ calendarconnection.OAuthConnector = (*FakeOAuthClient)(nil)
