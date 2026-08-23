package googlecalendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
)

type CalendarEventPublisher struct {
	config              Config
	credentialProtector calendarconnection.CredentialProtector
	httpClient          *http.Client
}

func NewCalendarEventPublisher(
	config Config,
	credentialProtector calendarconnection.CredentialProtector,
) (*CalendarEventPublisher, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	apiURL, err := url.ParseRequestURI(config.calendarAPIURL())
	if err != nil || apiURL.Scheme == "" || apiURL.Host == "" {
		return nil, fmt.Errorf("%w: invalid calendar API URL", ErrInvalidOAuthConfiguration)
	}
	return &CalendarEventPublisher{
		config:              config,
		credentialProtector: credentialProtector,
		httpClient:          &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (publisher *CalendarEventPublisher) Create(
	ctx context.Context,
	connection *calendarconnection.Connection,
	appointment workordercalendar.Appointment,
) (workordercalendar.PublishedEvent, error) {
	refreshToken, err := publisher.credentialProtector.Decrypt(connection.RefreshTokenCiphertext())
	if err != nil {
		return workordercalendar.PublishedEvent{}, fmt.Errorf("decrypting calendar refresh token: %w", err)
	}
	accessToken, err := publisher.refreshAccessToken(ctx, refreshToken)
	if err != nil {
		return workordercalendar.PublishedEvent{}, err
	}
	eventID, err := publisher.createEvent(ctx, accessToken, connection.CalendarID(), appointment)
	if err != nil {
		return workordercalendar.PublishedEvent{}, err
	}
	return workordercalendar.NewPublishedEvent(connection.CalendarID(), eventID)
}

func (publisher *CalendarEventPublisher) refreshAccessToken(ctx context.Context, refreshToken string) (string, error) {
	form := url.Values{
		"client_id":     []string{publisher.config.ClientID},
		"client_secret": []string{publisher.config.ClientSecret},
		"refresh_token": []string{refreshToken},
		"grant_type":    []string{"refresh_token"},
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		publisher.config.TokenURL,
		bytes.NewBufferString(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("creating Google Calendar refresh request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := publisher.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("requesting Google Calendar access token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: token request returned status %d", ErrCalendarEventTokenUnavailable, response.StatusCode)
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("decoding Google Calendar access token: %w", err)
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return "", ErrCalendarEventTokenUnavailable
	}
	return tokenResponse.AccessToken, nil
}

func (publisher *CalendarEventPublisher) createEvent(
	ctx context.Context,
	accessToken string,
	calendarID string,
	appointment workordercalendar.Appointment,
) (string, error) {
	payload := struct {
		Summary      string `json:"summary"`
		Description  string `json:"description"`
		Visibility   string `json:"visibility"`
		Transparency string `json:"transparency"`
		Start        struct {
			DateTime time.Time `json:"dateTime"`
		} `json:"start"`
		End struct {
			DateTime time.Time `json:"dateTime"`
		} `json:"end"`
		Reminders struct {
			UseDefault bool `json:"useDefault"`
		} `json:"reminders"`
	}{
		Summary:      "Servicio de LoResuelvo",
		Description:  fmt.Sprintf("Con: %s\n\n%s", appointment.CounterpartName(), appointment.Description()),
		Visibility:   "private",
		Transparency: "opaque",
	}
	payload.Start.DateTime = appointment.ScheduledOn()
	payload.End.DateTime = appointment.EndsOn()
	payload.Reminders.UseDefault = true
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encoding Google Calendar event: %w", err)
	}
	eventURL := strings.TrimRight(publisher.config.calendarAPIURL(), "/") +
		"/calendars/" + url.PathEscape(calendarID) + "/events"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, eventURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating Google Calendar event request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := publisher.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("requesting Google Calendar event creation: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: event request returned status %d", ErrCalendarEventCreationFailed, response.StatusCode)
	}
	var eventResponse struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&eventResponse); err != nil {
		return "", fmt.Errorf("decoding Google Calendar event: %w", err)
	}
	if strings.TrimSpace(eventResponse.ID) == "" {
		return "", ErrCalendarEventCreationFailed
	}
	return eventResponse.ID, nil
}

func (publisher *CalendarEventPublisher) client() *http.Client {
	if publisher.httpClient != nil {
		return publisher.httpClient
	}
	return http.DefaultClient
}

var _ workordercalendar.EventPublisher = (*CalendarEventPublisher)(nil)
