package repositories_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkOrderCalendarEventRepositoryStoresSynchronizedEvent(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "calendar-event")
	order := saveScheduledWorkOrderAt(
		t,
		testContext,
		fixture.conversation,
		fixture.consumerID,
		fixture.providerID,
		time.Now().UTC().Truncate(time.Microsecond).Add(48*time.Hour),
	)
	event, err := workordercalendar.NewEvent(order, order.Consumer())
	require.NoError(t, err)
	published, err := workordercalendar.NewPublishedEvent("primary", "event-40-10")
	require.NoError(t, err)
	syncedOn := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, event.MarkSynced(published, syncedOn))
	repository := repositories.NewWorkOrderCalendarEventRepository(testContext.database)

	err = repository.Save(t.Context(), event)

	require.NoError(t, err)
	var calendarID, googleEventID string
	var storedSyncedOn time.Time
	var attemptCount int
	require.NoError(t, testContext.database.QueryRow(
		`SELECT calendar_id, google_event_id, synced_on, attempt_count
		FROM work_order_calendar_events
		WHERE work_order_id = $1 AND user_id = $2`,
		order.ID(),
		order.Consumer().ID(),
	).Scan(&calendarID, &googleEventID, &storedSyncedOn, &attemptCount))
	assert.Equal(t, "primary", calendarID)
	assert.Equal(t, "event-40-10", googleEventID)
	assert.True(t, storedSyncedOn.Equal(syncedOn))
	assert.Zero(t, attemptCount)
}
