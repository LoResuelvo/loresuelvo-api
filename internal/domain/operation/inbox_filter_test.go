package operation_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

func TestInboxFilterRejectsInvalidDimensions(t *testing.T) {
	zero, tooLarge := 0, math.MaxInt32+1
	from := time.Date(2026, 9, 21, 3, 0, 0, 0, time.UTC)
	to := from.Add(-24 * time.Hour)
	stage, alert := readmodel.Stage("expired"), readmodel.Alert("expired")
	for name, filter := range map[string]operation.InboxFilter{
		"zero consumer":       {ConsumerID: &zero},
		"oversized provider":  {ProviderID: &tooLarge},
		"zero category":       {CategoryID: &zero},
		"inverted range":      {StartedFrom: &from, StartedTo: &to},
		"empty range":         {StartedFrom: &from, StartedTo: &from},
		"unknown stage":       {Stage: &stage},
		"unknown alert":       {Alert: &alert},
		"impossible calendar": {ScheduledDay: &operation.CalendarDay{Year: 2026, Month: time.September, Day: 31}},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, filter.Validate(), operation.ErrInvalidInboxQuery)
		})
	}
}

func TestCalendarDayBoundsFollowBuenosAires(t *testing.T) {
	from, to := operation.CalendarDay{Year: 2026, Month: time.September, Day: 25}.Bounds()

	require.Equal(t, time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC), from.UTC())
	require.Equal(t, time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC), to.UTC())
}

func TestInboxServiceTurnsTheScheduledDayIntoUTCInstants(t *testing.T) {
	ctx := context.Background()
	day := operation.CalendarDay{Year: 2026, Month: time.September, Day: 25}
	from, to := time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC), time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC)
	criteria := inboxCriteria(operation.DefaultInboxLimit + 1)
	criteria.Filter = operation.InboxFilter{ScheduledDay: &day}
	criteria.ScheduledWindow = &operation.TimeWindow{From: from, To: to}
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, criteria).Return(nil, nil).Once()

	_, err := inboxService(reader).Query(ctx, operation.InboxQuery{Filter: operation.InboxFilter{ScheduledDay: &day}})

	require.NoError(t, err)
	reader.AssertExpectations(t)
}

func TestInboxQueryRejectsInvalidPositions(t *testing.T) {
	startedOn := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	for name, position := range map[string]operation.InboxPosition{
		"unknown kind":      {StartedOn: startedOn, ID: readmodel.ID{Kind: "wo", ResourceID: 1}},
		"zero resource":     {StartedOn: startedOn, ID: readmodel.ID{Kind: readmodel.KindJobRequest}},
		"oversized ID":      {StartedOn: startedOn, ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: math.MaxInt32 + 1}},
		"missing timestamp": {ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, operation.InboxQuery{After: &position}.Validate(), operation.ErrInvalidInboxQuery)
		})
	}
}
