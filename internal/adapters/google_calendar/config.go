package googlecalendar

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	defaultAuthorizationURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL          = "https://oauth2.googleapis.com/token"
	defaultCalendarAPIURL    = "https://www.googleapis.com/calendar/v3"
	calendarEventsOwnedScope = "https://www.googleapis.com/auth/calendar.events.owned"
	primaryCalendarID        = "primary"
)

type Config struct {
	ClientID         string
	ClientSecret     string
	RedirectURI      string
	AuthorizationURL string
	TokenURL         string
	CalendarAPIURL   string
}

func (config Config) Validate() error {
	for _, value := range []string{config.ClientID, config.ClientSecret, config.RedirectURI} {
		if strings.TrimSpace(value) == "" {
			return ErrInvalidOAuthConfiguration
		}
	}
	for _, rawURL := range []string{config.RedirectURI, config.AuthorizationURL, config.TokenURL} {
		parsedURL, err := url.ParseRequestURI(rawURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("%w: invalid URL", ErrInvalidOAuthConfiguration)
		}
	}
	return nil
}

func NewConfigFromEnv() (Config, error) {
	config := Config{
		ClientID:         strings.TrimSpace(os.Getenv("GOOGLE_CALENDAR_CLIENT_ID")),
		ClientSecret:     strings.TrimSpace(os.Getenv("GOOGLE_CALENDAR_CLIENT_SECRET")),
		RedirectURI:      strings.TrimSpace(os.Getenv("GOOGLE_CALENDAR_REDIRECT_URI")),
		AuthorizationURL: envOrDefault("GOOGLE_CALENDAR_AUTHORIZATION_URL", defaultAuthorizationURL),
		TokenURL:         envOrDefault("GOOGLE_CALENDAR_TOKEN_URL", defaultTokenURL),
		CalendarAPIURL:   envOrDefault("GOOGLE_CALENDAR_API_URL", defaultCalendarAPIURL),
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) calendarAPIURL() string {
	if strings.TrimSpace(config.CalendarAPIURL) == "" {
		return defaultCalendarAPIURL
	}
	return strings.TrimSpace(config.CalendarAPIURL)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
