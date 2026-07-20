package paymentaccount

import (
	"context"
	"strings"
	"time"
)

type OAuthConnector interface {
	Provider() PaymentProvider
	AuthorizationURL(state, codeVerifier string) (string, error)
	ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (OAuthCredentials, error)
}

type OAuthCredentials struct {
	ExternalAccountID string
	AccessToken       string
	RefreshToken      string
	ExpiresOn         time.Time
}

func ValidateOAuthCredentials(credentials OAuthCredentials) error {
	if strings.TrimSpace(credentials.ExternalAccountID) == "" {
		return ErrExternalAccountIDRequired
	}
	if strings.TrimSpace(credentials.AccessToken) == "" {
		return ErrAccessTokenRequired
	}
	return nil
}
