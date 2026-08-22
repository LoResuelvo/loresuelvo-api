package calendar_connection_handler

import "os"

const (
	defaultConnectionSuccessURL   = "/me"
	defaultConnectionCancelledURL = "/me"
)

type Config struct {
	ConnectionSuccessURL   string
	ConnectionCancelledURL string
}

func NewConfigFromEnv() Config {
	return Config{
		ConnectionSuccessURL:   envOrDefault("GOOGLE_CALENDAR_CONNECTION_SUCCESS_URL", defaultConnectionSuccessURL),
		ConnectionCancelledURL: envOrDefault("GOOGLE_CALENDAR_CONNECTION_CANCELLED_URL", defaultConnectionCancelledURL),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
