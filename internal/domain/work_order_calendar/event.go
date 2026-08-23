package workordercalendar

import (
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

// EventKey is the persistence identity of a participant's appointment.
// A work order has one independently synchronized event per participant.
type EventKey struct {
	workOrderID   int
	participantID int
}

func NewEventKey(workOrderID, participantID int) (EventKey, error) {
	if workOrderID <= 0 || participantID <= 0 {
		return EventKey{}, ErrCalendarEventIdentity
	}
	return EventKey{workOrderID: workOrderID, participantID: participantID}, nil
}

func (key EventKey) WorkOrderID() int {
	return key.workOrderID
}

func (key EventKey) ParticipantID() int {
	return key.participantID
}

type PublishedEvent struct {
	calendarID string
	externalID string
}

func NewPublishedEvent(calendarID, externalID string) (PublishedEvent, error) {
	calendarID = strings.TrimSpace(calendarID)
	if calendarID == "" {
		return PublishedEvent{}, ErrCalendarEventCalendarID
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return PublishedEvent{}, ErrCalendarEventExternalID
	}
	return PublishedEvent{calendarID: calendarID, externalID: externalID}, nil
}

func (published PublishedEvent) CalendarID() string {
	return published.calendarID
}

func (published PublishedEvent) ExternalID() string {
	return published.externalID
}

type Event struct {
	key       EventKey
	published PublishedEvent
	syncedOn  time.Time
}

func NewEvent(order *workorder.WorkOrder, participant user.User) (*Event, error) {
	key, err := NewEventKey(order.ID(), participant.ID())
	if err != nil {
		return nil, err
	}
	return &Event{key: key}, nil
}

func RehydrateEvent(key EventKey, calendarID, externalID string, syncedOn time.Time) (*Event, error) {
	if key.workOrderID <= 0 || key.participantID <= 0 {
		return nil, ErrCalendarEventIdentity
	}
	if syncedOn.IsZero() {
		if calendarID != "" {
			return nil, ErrCalendarEventCalendarID
		}
		if externalID != "" {
			return nil, ErrCalendarEventExternalID
		}
		return &Event{key: key}, nil
	}
	published, err := NewPublishedEvent(calendarID, externalID)
	if err != nil {
		return nil, err
	}
	return &Event{key: key, published: published, syncedOn: syncedOn.UTC()}, nil
}

func (event *Event) Key() EventKey {
	return event.key
}

func (event *Event) Published() PublishedEvent {
	return event.published
}

func (event *Event) SyncedOn() time.Time {
	return event.syncedOn
}

func (event *Event) IsSynced() bool {
	return !event.syncedOn.IsZero()
}

func (event *Event) MarkSynced(published PublishedEvent, syncedOn time.Time) error {
	if !event.syncedOn.IsZero() {
		return ErrCalendarEventAlreadySynced
	}
	if syncedOn.IsZero() {
		return ErrCalendarEventSyncedOn
	}
	if published.calendarID == "" || published.externalID == "" {
		return ErrCalendarEventIdentity
	}
	event.published = published
	event.syncedOn = syncedOn.UTC()
	return nil
}
