package mercadopago

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

var ErrInvalidCheckoutConfiguration = errors.New("Mercado Pago checkout configuration is incomplete")

type Config struct {
	SuccessURL      string
	PendingURL      string
	FailureURL      string
	NotificationURL string
}

func NewConfigFromEnv() Config {
	return Config{
		SuccessURL:      strings.TrimSpace(os.Getenv("PAYMENT_CHECKOUT_SUCCESS_URL")),
		PendingURL:      strings.TrimSpace(os.Getenv("PAYMENT_CHECKOUT_PENDING_URL")),
		FailureURL:      strings.TrimSpace(os.Getenv("PAYMENT_CHECKOUT_FAILURE_URL")),
		NotificationURL: strings.TrimSpace(os.Getenv("MERCADO_PAGO_NOTIFICATION_URL")),
	}
}

func (config Config) Validate() error {
	publicHTTPSURLs := []string{
		config.SuccessURL,
		config.PendingURL,
		config.FailureURL,
		config.NotificationURL,
	}
	for _, rawURL := range publicHTTPSURLs {
		parsedURL, parseErr := url.ParseRequestURI(strings.TrimSpace(rawURL))
		if parseErr != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return fmt.Errorf("%w: callbacks must be absolute HTTPS URLs", ErrInvalidCheckoutConfiguration)
		}
	}
	return nil
}
