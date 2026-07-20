package mercadopago

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	defaultAuthorizationBaseURL = "https://auth.mercadopago.com/authorization"
	defaultAPIBaseURL           = "https://api.mercadopago.com"
)

type Config struct {
	ClientID             string
	ClientSecret         string
	RedirectURI          string
	AuthorizationBaseURL string
	APIBaseURL           string
}

func (config Config) Validate() error {
	requiredValues := []string{
		config.ClientID,
		config.ClientSecret,
		config.RedirectURI,
		config.AuthorizationBaseURL,
		config.APIBaseURL,
	}
	for _, value := range requiredValues {
		if strings.TrimSpace(value) == "" {
			return ErrInvalidOAuthConfiguration
		}
	}
	for _, rawURL := range []string{
		config.RedirectURI,
		config.AuthorizationBaseURL,
		config.APIBaseURL,
	} {
		parsedURL, err := url.ParseRequestURI(rawURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("%w: invalid URL", ErrInvalidOAuthConfiguration)
		}
	}
	return nil
}

func NewConfigFromEnv() Config {
	return Config{
		ClientID:             strings.TrimSpace(os.Getenv("MERCADO_PAGO_CLIENT_ID")),
		ClientSecret:         strings.TrimSpace(os.Getenv("MERCADO_PAGO_CLIENT_SECRET")),
		RedirectURI:          strings.TrimSpace(os.Getenv("MERCADO_PAGO_REDIRECT_URI")),
		AuthorizationBaseURL: envOrDefault("MERCADO_PAGO_AUTHORIZATION_BASE_URL", defaultAuthorizationBaseURL),
		APIBaseURL:           envOrDefault("MERCADO_PAGO_API_BASE_URL", defaultAPIBaseURL),
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
