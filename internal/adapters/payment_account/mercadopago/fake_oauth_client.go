package mercadopago

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

var ErrFakeAuthorizationCodeInvalid = errors.New("Mercado Pago authorization code is invalid")

type FakeOAuthClient struct{}

func NewFakeOAuthClient() *FakeOAuthClient {
	return &FakeOAuthClient{}
}

func (client *FakeOAuthClient) Provider() paymentaccount.PaymentProvider {
	return paymentProvider
}

func (client *FakeOAuthClient) AuthorizationURL(state, codeVerifier string) (string, error) {
	authorizationURL := &url.URL{
		Scheme: "https",
		Host:   "auth.mercadopago.test",
		Path:   "/authorization",
	}
	query := authorizationURL.Query()
	query.Set("state", state)
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	return authorizationURL.String(), nil
}

func (client *FakeOAuthClient) ExchangeAuthorizationCode(_ context.Context, code, _ string) (paymentaccount.OAuthCredentials, error) {
	const enabledPrefix = "marketplace-enabled:"
	if !strings.HasPrefix(code, enabledPrefix) {
		return paymentaccount.OAuthCredentials{}, ErrFakeAuthorizationCodeInvalid
	}
	externalAccountID := strings.TrimPrefix(code, enabledPrefix)
	return paymentaccount.OAuthCredentials{
		ExternalAccountID:             externalAccountID,
		AccessToken:                   "fake-access-token-for-" + externalAccountID,
		RefreshToken:                  "fake-refresh-token-for-" + externalAccountID,
		ExpiresOn:                     time.Now().UTC().Add(180 * 24 * time.Hour),
		CanReceiveMarketplacePayments: true,
	}, nil
}
