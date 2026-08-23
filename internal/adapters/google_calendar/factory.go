package googlecalendar

import (
	"fmt"
	"os"
	"strings"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
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

func NewEventPublisherFromEnv(
	credentialProtector calendarconnection.CredentialProtector,
) (workordercalendar.EventPublisher, error) {
	config, err := NewConfigFromEnv()
	if err == nil {
		return NewCalendarEventPublisher(config, credentialProtector)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "dev") {
		return NewFakeEventPublisher(), nil
	}
	return nil, fmt.Errorf("configuring Google Calendar event publisher: %w", err)
}
