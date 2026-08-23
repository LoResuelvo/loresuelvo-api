package googlecalendar

import (
	"context"
	"fmt"
	"sync"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
)

type fakeEventKey struct {
	userID      int
	workOrderID int
}

type FakeEventPublisher struct {
	mu     sync.RWMutex
	events map[fakeEventKey]string
}

func NewFakeEventPublisher() *FakeEventPublisher {
	return &FakeEventPublisher{events: make(map[fakeEventKey]string)}
}

func (publisher *FakeEventPublisher) Create(
	_ context.Context,
	connection *calendarconnection.Connection,
	appointment workordercalendar.Appointment,
) (workordercalendar.PublishedEvent, error) {
	key := fakeEventKey{
		userID:      appointment.Participant().ID(),
		workOrderID: appointment.WorkOrder().ID(),
	}
	externalID := fmt.Sprintf("fake-event-%d-%d", key.workOrderID, key.userID)
	publisher.mu.Lock()
	publisher.events[key] = externalID
	publisher.mu.Unlock()
	return workordercalendar.NewPublishedEvent(connection.CalendarID(), externalID)
}

func (publisher *FakeEventPublisher) HasEventForUser(_ context.Context, userID, workOrderID int) (bool, error) {
	publisher.mu.RLock()
	_, found := publisher.events[fakeEventKey{userID: userID, workOrderID: workOrderID}]
	publisher.mu.RUnlock()
	return found, nil
}

var _ workordercalendar.EventPublisher = (*FakeEventPublisher)(nil)
