package workordercalendar_test

import (
	"testing"
	"time"

	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventDerivesItsPersistenceIdentityFromDomainObjects(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, _ := workOrderCalendarFixture(t, now)

	event, err := workordercalendar.NewEvent(order, consumerUser)

	require.NoError(t, err)
	assert.Equal(t, order.ID(), event.Key().WorkOrderID())
	assert.Equal(t, consumerUser.ID(), event.Key().ParticipantID())
	assert.False(t, event.IsSynced())
}

func TestEventMarksThePublishedEventAsSynchronized(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, _ := workOrderCalendarFixture(t, now)
	event, err := workordercalendar.NewEvent(order, consumerUser)
	require.NoError(t, err)
	published := publishedEventFixture(t, "event-40-10")

	err = event.MarkSynced(published, now)

	require.NoError(t, err)
	assert.True(t, event.IsSynced())
	assert.Equal(t, published, event.Published())
	assert.Equal(t, now, event.SyncedOn())
}

func TestEventRejectsASecondSynchronization(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	key, err := workordercalendar.NewEventKey(40, 10)
	require.NoError(t, err)
	event, err := workordercalendar.RehydrateEvent(key, "primary", "event-40-10", now)
	require.NoError(t, err)

	err = event.MarkSynced(publishedEventFixture(t, "replacement"), now.Add(time.Minute))

	assert.ErrorIs(t, err, workordercalendar.ErrCalendarEventAlreadySynced)
	assert.Equal(t, "event-40-10", event.Published().ExternalID())
}

func TestPublishedEventRequiresExternalIdentities(t *testing.T) {
	tests := []struct {
		name       string
		calendarID string
		externalID string
		expected   error
	}{
		{name: "calendar identity", externalID: "event-40-10", expected: workordercalendar.ErrCalendarEventCalendarID},
		{name: "external identity", calendarID: "primary", expected: workordercalendar.ErrCalendarEventExternalID},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := workordercalendar.NewPublishedEvent(test.calendarID, test.externalID)

			assert.ErrorIs(t, err, test.expected)
		})
	}
}
