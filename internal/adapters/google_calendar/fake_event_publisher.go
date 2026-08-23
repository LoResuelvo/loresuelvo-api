package googlecalendar

import (
	"context"
	"fmt"
	"sync"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
)

type fakeEventKey struct {
	userID      int
	workOrderID int
}

type EventDetails struct {
	CalendarID   string
	ExternalID   string
	Summary      string
	Description  string
	Start        time.Time
	End          time.Time
	Visibility   string
	Transparency string
}

type FakeEventPublisher struct {
	mu        sync.RWMutex
	available bool
	events    map[fakeEventKey]EventDetails
}

func NewFakeEventPublisher() *FakeEventPublisher {
	return &FakeEventPublisher{available: true, events: make(map[fakeEventKey]EventDetails)}
}

func (publisher *FakeEventPublisher) SetAvailable(available bool) {
	publisher.mu.Lock()
	publisher.available = available
	publisher.mu.Unlock()
}

func (publisher *FakeEventPublisher) Create(
	_ context.Context,
	connection *calendarconnection.Connection,
	appointment workordercalendar.Appointment,
) (workordercalendar.PublishedEvent, error) {
	publisher.mu.RLock()
	available := publisher.available
	publisher.mu.RUnlock()
	if !available {
		return workordercalendar.PublishedEvent{}, fmt.Errorf("Google Calendar is unavailable")
	}
	key := fakeEventKey{
		userID:      appointment.Participant().ID(),
		workOrderID: appointment.WorkOrder().ID(),
	}
	externalID := fmt.Sprintf("fake-event-%d-%d", key.workOrderID, key.userID)
	details := EventDetails{
		CalendarID:   connection.CalendarID(),
		ExternalID:   externalID,
		Summary:      "Servicio de LoResuelvo",
		Description:  fmt.Sprintf("Con: %s\n\n%s", appointment.CounterpartName(), appointment.Description()),
		Start:        appointment.ScheduledOn(),
		End:          appointment.EndsOn(),
		Visibility:   "private",
		Transparency: "opaque",
	}
	publisher.mu.Lock()
	publisher.events[key] = details
	publisher.mu.Unlock()
	return workordercalendar.NewPublishedEvent(connection.CalendarID(), externalID)
}

func (publisher *FakeEventPublisher) HasEventForUser(_ context.Context, userID, workOrderID int) (bool, error) {
	publisher.mu.RLock()
	_, found := publisher.events[fakeEventKey{userID: userID, workOrderID: workOrderID}]
	publisher.mu.RUnlock()
	return found, nil
}

func (publisher *FakeEventPublisher) EventDetailsForUser(_ context.Context, userID, workOrderID int) (EventDetails, bool, error) {
	publisher.mu.RLock()
	details, found := publisher.events[fakeEventKey{userID: userID, workOrderID: workOrderID}]
	publisher.mu.RUnlock()
	return details, found, nil
}

var _ workordercalendar.EventPublisher = (*FakeEventPublisher)(nil)
