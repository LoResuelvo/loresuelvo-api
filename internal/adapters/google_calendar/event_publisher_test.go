package googlecalendar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarEventPublisherRefreshesCredentialsAndCreatesAnEvent(t *testing.T) {
	now := time.Date(2026, time.August, 15, 15, 0, 0, 0, time.UTC)
	appointment, connection := calendarAppointmentFixture(t, now)
	var receivedTokenForm url.Values
	var receivedEventPath, receivedAuthorization string
	var receivedEventBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			receivedTokenForm, err = url.ParseQuery(string(body))
			require.NoError(t, err)
			response.Header().Set("Content-Type", "application/json")
			_, err = response.Write([]byte(`{"access_token":"access-token"}`))
			require.NoError(t, err)
		case "/calendar/v3/calendars/primary/events":
			receivedEventPath = request.URL.Path
			receivedAuthorization = request.Header.Get("Authorization")
			receivedEventBody, _ = io.ReadAll(request.Body)
			response.Header().Set("Content-Type", "application/json")
			_, err := response.Write([]byte(`{"id":"google-event-40-10"}`))
			require.NoError(t, err)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	protector := new(credentialProtectorMock)
	protector.On("Decrypt", []byte("encrypted-refresh-token")).Return("refresh-token", nil).Once()
	config := testOAuthConfig(server.URL)
	config.CalendarAPIURL = server.URL + "/calendar/v3"
	publisher, err := NewCalendarEventPublisher(config, protector)
	require.NoError(t, err)
	publisher.httpClient = server.Client()

	published, err := publisher.Create(context.Background(), connection, appointment)

	require.NoError(t, err)
	assert.Equal(t, "primary", published.CalendarID())
	assert.Equal(t, "google-event-40-10", published.ExternalID())
	assert.Equal(t, "calendar-client-id", receivedTokenForm.Get("client_id"))
	assert.Equal(t, "calendar-client-secret", receivedTokenForm.Get("client_secret"))
	assert.Equal(t, "refresh-token", receivedTokenForm.Get("refresh_token"))
	assert.Equal(t, "refresh_token", receivedTokenForm.Get("grant_type"))
	assert.Equal(t, "/calendar/v3/calendars/primary/events", receivedEventPath)
	assert.Equal(t, "Bearer access-token", receivedAuthorization)
	var eventPayload struct {
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
	}
	require.NoError(t, json.Unmarshal(receivedEventBody, &eventPayload))
	assert.Equal(t, "Servicio de LoResuelvo", eventPayload.Summary)
	assert.Contains(t, eventPayload.Description, appointment.CounterpartName())
	assert.Contains(t, eventPayload.Description, appointment.Description())
	assert.Equal(t, "private", eventPayload.Visibility)
	assert.Equal(t, "opaque", eventPayload.Transparency)
	assert.True(t, eventPayload.Reminders.UseDefault)
	assert.True(t, eventPayload.Start.DateTime.Equal(appointment.ScheduledOn()))
	assert.True(t, eventPayload.End.DateTime.Equal(appointment.EndsOn()))
	protector.AssertExpectations(t)
}

func TestFakeEventPublisherExposesPublishedAppointment(t *testing.T) {
	now := time.Date(2026, time.August, 15, 15, 0, 0, 0, time.UTC)
	appointment, connection := calendarAppointmentFixture(t, now)
	publisher := NewFakeEventPublisher()

	published, err := publisher.Create(context.Background(), connection, appointment)

	require.NoError(t, err)
	assert.Equal(t, connection.CalendarID(), published.CalendarID())
	hasEvent, err := publisher.HasEventForUser(context.Background(), appointment.Participant().ID(), appointment.WorkOrder().ID())
	require.NoError(t, err)
	assert.True(t, hasEvent)
	otherEvent, err := publisher.HasEventForUser(context.Background(), appointment.Counterpart().ID(), appointment.WorkOrder().ID())
	require.NoError(t, err)
	assert.False(t, otherEvent)
	details, found, err := publisher.EventDetailsForUser(context.Background(), appointment.Participant().ID(), appointment.WorkOrder().ID())
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, connection.CalendarID(), details.CalendarID)
	assert.Equal(t, published.ExternalID(), details.ExternalID)
	assert.Equal(t, "Servicio de LoResuelvo", details.Summary)
	assert.Contains(t, details.Description, appointment.CounterpartName())
	assert.Contains(t, details.Description, appointment.Description())
	assert.True(t, details.Start.Equal(appointment.ScheduledOn()))
	assert.True(t, details.End.Equal(appointment.EndsOn()))
	assert.Equal(t, "private", details.Visibility)
	assert.Equal(t, "opaque", details.Transparency)
	count, err := publisher.EventCountForUser(context.Background(), appointment.Participant().ID(), appointment.WorkOrder().ID())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func calendarAppointmentFixture(t *testing.T, scheduledOn time.Time) (workordercalendar.Appointment, *calendarconnection.Connection) {
	t.Helper()
	consumerUser := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(10, "auth0|consumer", "ana@example.com", "Ana", "Pérez", consumer.Role, nil)}
	providerUser := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "juan@example.com", "Juan", "Gómez", provider.Role, nil)}
	proposal := &serviceproposal.ServiceProposal{
		ID:                       30,
		Consumer:                 consumerUser,
		Provider:                 providerUser,
		ScheduledOn:              scheduledOn,
		Description:              "Repair the kitchen water leak.",
		EstimatedDurationMinutes: 90,
	}
	order, err := workorder.New(proposal, scheduledOn.Add(-time.Hour))
	require.NoError(t, err)
	order.SetID(40)
	connection, err := calendarconnection.NewConnection(consumerUser.ID(), primaryCalendarID, []byte("encrypted-refresh-token"), scheduledOn)
	require.NoError(t, err)
	return workordercalendar.NewAppointment(order, consumerUser, providerUser), connection
}
