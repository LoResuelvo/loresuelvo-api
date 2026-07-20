package payment_account_handler

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

var ErrInvalidConfiguration = errors.New("payment account HTTP configuration is incomplete")

type Config struct {
	ConnectionSuccessURL string
}

func NewConfigFromEnv() (Config, error) {
	config := Config{
		ConnectionSuccessURL: strings.TrimSpace(os.Getenv("PAYMENT_ACCOUNT_CONNECTION_SUCCESS_URL")),
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	parsedURL, err := url.ParseRequestURI(config.ConnectionSuccessURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%w: invalid connection success URL", ErrInvalidConfiguration)
	}
	return nil
}
