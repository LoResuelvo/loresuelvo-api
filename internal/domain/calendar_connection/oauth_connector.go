package calendarconnection

import (
	"context"
	"strings"
)

type OAuthConnector interface {
	AuthorizationURL(state, codeVerifier string) (string, error)
	ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (AuthorizationCredentials, error)
}

type AuthorizationCredentials struct {
	CalendarID   string
	RefreshToken string
}

func ValidateAuthorizationCredentials(credentials AuthorizationCredentials) error {
	if strings.TrimSpace(credentials.CalendarID) == "" {
		return ErrCalendarIDRequired
	}
	if strings.TrimSpace(credentials.RefreshToken) == "" {
		return ErrRefreshTokenRequired
	}
	return nil
}
