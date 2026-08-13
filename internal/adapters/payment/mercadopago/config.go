package mercadopago

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	sharedmercadopago "github.com/LoResuelvo/loresuelvo-api/internal/adapters/mercadopago"
)

var ErrInvalidCheckoutConfiguration = errors.New("Mercado Pago checkout configuration is incomplete")

const webhookNotificationSource = "webhooks"

type Config struct {
	Environment     sharedmercadopago.Environment
	SuccessURL      string
	PendingURL      string
	FailureURL      string
	NotificationURL string
}

func NewConfigFromEnv() (Config, error) {
	environment, err := sharedmercadopago.EnvironmentFromEnv()
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalidCheckoutConfiguration, err)
	}
	return Config{
		Environment:     environment,
		SuccessURL:      strings.TrimSpace(os.Getenv("PAYMENT_CHECKOUT_SUCCESS_URL")),
		PendingURL:      strings.TrimSpace(os.Getenv("PAYMENT_CHECKOUT_PENDING_URL")),
		FailureURL:      strings.TrimSpace(os.Getenv("PAYMENT_CHECKOUT_FAILURE_URL")),
		NotificationURL: strings.TrimSpace(os.Getenv("MERCADO_PAGO_NOTIFICATION_URL")),
	}, nil
}

func (config Config) Validate() error {
	if err := config.Environment.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCheckoutConfiguration, err)
	}
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

func (config Config) withWebhookNotificationURL() (Config, error) {
	notificationURL, err := url.ParseRequestURI(strings.TrimSpace(config.NotificationURL))
	if err != nil {
		return Config{}, fmt.Errorf("%w: notification URL is invalid", ErrInvalidCheckoutConfiguration)
	}
	query := notificationURL.Query()
	query.Set("source_news", webhookNotificationSource)
	notificationURL.RawQuery = query.Encode()
	config.NotificationURL = notificationURL.String()
	return config, nil
}
