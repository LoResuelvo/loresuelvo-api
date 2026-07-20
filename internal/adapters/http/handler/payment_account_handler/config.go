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
	ConnectionSuccessURL   string
	ConnectionCancelledURL string
}

func NewConfigFromEnv() (Config, error) {
	config := Config{
		ConnectionSuccessURL:   strings.TrimSpace(os.Getenv("PAYMENT_ACCOUNT_CONNECTION_SUCCESS_URL")),
		ConnectionCancelledURL: strings.TrimSpace(os.Getenv("PAYMENT_ACCOUNT_CONNECTION_CANCELLED_URL")),
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	if !isAbsoluteURL(config.ConnectionSuccessURL) {
		return fmt.Errorf("%w: invalid connection success URL", ErrInvalidConfiguration)
	}
	if !isAbsoluteURL(config.ConnectionCancelledURL) {
		return fmt.Errorf("%w: invalid connection cancelled URL", ErrInvalidConfiguration)
	}
	return nil
}

func isAbsoluteURL(value string) bool {
	parsedURL, err := url.ParseRequestURI(value)
	return err == nil && parsedURL.Scheme != "" && parsedURL.Host != ""
}
