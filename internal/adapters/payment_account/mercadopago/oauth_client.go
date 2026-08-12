package mercadopago

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

var ErrInvalidOAuthConfiguration = errors.New("Mercado Pago OAuth configuration is incomplete")

const paymentProvider = paymentaccount.PaymentProvider("mercado_pago")

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
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func NewOAuthClientFromEnv() (*OAuthClient, error) {
	config, err := NewConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewOAuthClient(config)
}

func (client *OAuthClient) Provider() paymentaccount.PaymentProvider {
	return paymentProvider
}

func (client *OAuthClient) AuthorizationURL(state, codeVerifier string) (string, error) {
	if client.config.ClientID == "" || client.config.RedirectURI == "" || client.config.AuthorizationBaseURL == "" {
		return "", ErrInvalidOAuthConfiguration
	}
	authorizationURL, err := url.Parse(client.config.AuthorizationBaseURL)
	if err != nil {
		return "", fmt.Errorf("parsing Mercado Pago authorization base URL: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("client_id", client.config.ClientID)
	query.Set("response_type", "code")
	query.Set("platform_id", "mp")
	query.Set("state", state)
	query.Set("redirect_uri", client.config.RedirectURI)
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	return authorizationURL.String(), nil
}

func pkceChallenge(codeVerifier string) string {
	digest := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (client *OAuthClient) ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (paymentaccount.OAuthCredentials, error) {
	if client.config.ClientID == "" || client.config.ClientSecret == "" || client.config.RedirectURI == "" || client.config.APIBaseURL == "" {
		return paymentaccount.OAuthCredentials{}, ErrInvalidOAuthConfiguration
	}
	payload, err := json.Marshal(struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		GrantType    string `json:"grant_type"`
		Code         string `json:"code"`
		RedirectURI  string `json:"redirect_uri"`
		CodeVerifier string `json:"code_verifier"`
		TestToken    bool   `json:"test_token"`
	}{
		ClientID:     client.config.ClientID,
		ClientSecret: client.config.ClientSecret,
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  client.config.RedirectURI,
		CodeVerifier: codeVerifier,
		TestToken:    client.config.Environment.IsSandbox(),
	})
	if err != nil {
		return paymentaccount.OAuthCredentials{}, fmt.Errorf("encoding Mercado Pago token request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(client.config.APIBaseURL, "/")+"/oauth/token",
		bytes.NewReader(payload),
	)
	if err != nil {
		return paymentaccount.OAuthCredentials{}, fmt.Errorf("creating Mercado Pago token request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return paymentaccount.OAuthCredentials{}, fmt.Errorf("requesting Mercado Pago token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var oauthError struct {
			Error string `json:"error"`
		}
		if decodeErr := json.NewDecoder(response.Body).Decode(&oauthError); decodeErr == nil {
			switch oauthError.Error {
			case "invalid_grant":
				return paymentaccount.OAuthCredentials{}, paymentaccount.ErrAuthorizationCodeUnusable
			case "unauthorized_client":
				return paymentaccount.OAuthCredentials{}, paymentaccount.ErrAuthorizationGrantUnavailable
			}
		}
		return paymentaccount.OAuthCredentials{}, fmt.Errorf("Mercado Pago token request returned status %d", response.StatusCode)
	}

	var tokenResponse struct {
		AccessToken  string      `json:"access_token"`
		RefreshToken string      `json:"refresh_token"`
		UserID       json.Number `json:"user_id"`
		ExpiresIn    int64       `json:"expires_in"`
	}
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&tokenResponse); err != nil {
		return paymentaccount.OAuthCredentials{}, fmt.Errorf("decoding Mercado Pago token response: %w", err)
	}
	externalAccountID := tokenResponse.UserID.String()
	if externalAccountID == "" {
		return paymentaccount.OAuthCredentials{}, paymentaccount.ErrExternalAccountIDRequired
	}
	if _, err := strconv.ParseInt(externalAccountID, 10, 64); err != nil {
		return paymentaccount.OAuthCredentials{}, fmt.Errorf("invalid Mercado Pago user id")
	}

	return paymentaccount.OAuthCredentials{
		ExternalAccountID: externalAccountID,
		AccessToken:       tokenResponse.AccessToken,
		RefreshToken:      tokenResponse.RefreshToken,
		ExpiresOn:         time.Now().UTC().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
	}, nil
}
