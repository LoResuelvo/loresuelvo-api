package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLogQueryServiceReadsBeforeAuditingAndReturnsImmutableEvents(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	listed, err := audit.NewEvent(audit.EventParams{
		ID: uuid.New(), OperatorID: 7, Action: audit.ActionCreate,
		ResourceType: "category", ResourceID: "17", OccurredOn: now.Add(-time.Hour),
		Result: audit.ResultSucceeded, CorrelationID: "existing-request",
	})
	require.NoError(t, err)
	operatorID := 7
	sequence := make([]string, 0, 2)
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	finder.On("FindOperatorIDByAuthID", ctx, "auth0|supervisor").Return(23, nil).Once()
	reader.On("FindLatest", ctx, audit.LogFilter{OperatorID: &operatorID}, 20).Run(func(mock.Arguments) {
		sequence = append(sequence, "read")
	}).Return([]*audit.Event{listed}, nil).Once()
	writer.On("Save", ctx, mock.MatchedBy(func(event *audit.Event) bool {
		return event != nil && event.ID() != uuid.Nil && event.OperatorID() == 23 &&
			event.Action() == audit.ActionAccess && event.ResourceType() == "audit_log" &&
			event.ResourceID() == "" && event.OccurredOn().Equal(now) &&
			event.Result() == audit.ResultPrepared && event.CorrelationID() == "request-audit" &&
			event.Reason() == nil && event.StateChange() == nil
	})).Run(func(mock.Arguments) {
		sequence = append(sequence, "save")
	}).Return(nil).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{now})
	events, err := service.Query(ctx, "auth0|supervisor", "request-audit", audit.LogFilter{OperatorID: &operatorID})
	require.NoError(t, err)
	require.Equal(t, []*audit.Event{listed}, events)
	require.Equal(t, []string{"read", "save"}, sequence)
	reader.AssertExpectations(t)
	writer.AssertExpectations(t)
	finder.AssertExpectations(t)
}

func TestLogQueryServiceReturnsNonNilEmptyCollection(t *testing.T) {
	ctx := context.Background()
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("FindLatest", ctx, audit.LogFilter{}, 20).Return(nil, nil).Once()
	writer.On("Save", ctx, mock.Anything).Return(nil).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	events, err := service.Query(ctx, "actor", "request-audit", audit.LogFilter{})
	require.NoError(t, err)
	require.NotNil(t, events)
	require.Empty(t, events)
	writer.AssertExpectations(t)
}

func TestLogQueryServiceRejectsOperatorResolutionFailureBeforeReading(t *testing.T) {
	ctx := context.Background()
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	want := errors.New("operator lookup failed")
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(0, want).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	events, err := service.Query(ctx, "actor", "request-audit", audit.LogFilter{})
	require.Nil(t, events)
	require.ErrorIs(t, err, want)
	reader.AssertNotCalled(t, "FindLatest", mock.Anything, mock.Anything, mock.Anything)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestLogQueryServiceDoesNotAuditFailedRead(t *testing.T) {
	ctx := context.Background()
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	want := errors.New("read failed")
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("FindLatest", ctx, audit.LogFilter{}, 20).Return(nil, want).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	events, err := service.Query(ctx, "actor", "request-audit", audit.LogFilter{})
	require.Nil(t, events)
	require.ErrorIs(t, err, want)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestLogQueryServiceDoesNotReturnEventsWhenAuditPersistenceFails(t *testing.T) {
	ctx := context.Background()
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	want := errors.New("write failed")
	reason, err := audit.NewReason("Private reconciliation detail")
	require.NoError(t, err)
	listed, err := audit.NewEvent(audit.EventParams{
		ID: uuid.New(), OperatorID: 7, Action: audit.ActionExecute,
		ResourceType: "payment", ResourceID: "42", OccurredOn: time.Now(),
		Result: audit.ResultSucceeded, CorrelationID: "existing-request", Reason: reason,
	})
	require.NoError(t, err)
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("FindLatest", ctx, audit.LogFilter{}, 20).Return([]*audit.Event{listed}, nil).Once()
	writer.On("Save", ctx, mock.Anything).Return(want).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	events, err := service.Query(ctx, "actor", "request-audit", audit.LogFilter{})
	require.Nil(t, events)
	require.ErrorIs(t, err, want)
	require.NotContains(t, err.Error(), reason.Text())
}

func TestLogQueryServiceDoesNotReturnEventsWhenAccessEvidenceIsInvalid(t *testing.T) {
	ctx := context.Background()
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("FindLatest", ctx, audit.LogFilter{}, 20).Return([]*audit.Event{}, nil).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	events, err := service.Query(ctx, "actor", "invalid correlation with spaces", audit.LogFilter{})
	require.Nil(t, events)
	require.ErrorIs(t, err, audit.ErrInvalidEvent)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestLogQueryServicePassesValidatedFiltersToReader(t *testing.T) {
	ctx := context.Background()
	operatorID := 7
	action := audit.ActionExecute
	resourceType := "payment"
	resourceID := "42"
	result := audit.ResultSucceeded
	from := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	filter := audit.LogFilter{
		OperatorID: &operatorID, Action: &action, ResourceType: &resourceType,
		ResourceID: &resourceID, Result: &result, OccurredFrom: &from, OccurredTo: &to,
	}
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("FindLatest", ctx, filter, 20).Return([]*audit.Event{}, nil).Once()
	writer.On("Save", ctx, mock.Anything).Return(nil).Once()

	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	_, err := service.Query(ctx, "actor", "request-audit", filter)
	require.NoError(t, err)
	reader.AssertExpectations(t)
	writer.AssertExpectations(t)
}

func TestLogQueryServiceRejectsInvalidFilterBeforeResolvingOperatorOrReading(t *testing.T) {
	invalidOperator := 0
	reader := &logReaderMock{}
	writer := &auditWriterMock{}
	finder := &auditOperatorIDFinderMock{}
	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})

	events, err := service.Query(context.Background(), "actor", "request-audit", audit.LogFilter{OperatorID: &invalidOperator})
	require.Nil(t, events)
	require.ErrorIs(t, err, audit.ErrInvalidQuery)
	finder.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
	reader.AssertNotCalled(t, "FindLatest", mock.Anything, mock.Anything, mock.Anything)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}
