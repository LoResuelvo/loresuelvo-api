package googlecalendar

import (
	"fmt"
	"os"
	"strings"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
)

func NewOAuthConnectorFromEnv() (calendarconnection.OAuthConnector, error) {
	config, err := NewConfigFromEnv()
	if err == nil {
		return NewOAuthClient(config)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "dev") {
		return NewFakeOAuthClient(), nil
	}
	return nil, fmt.Errorf("configuring Google Calendar OAuth: %w", err)
}
