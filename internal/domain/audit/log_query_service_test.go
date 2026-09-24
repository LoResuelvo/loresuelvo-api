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

func queryEvent(t *testing.T, at time.Time) *audit.Event {
	t.Helper()
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: 7, Action: audit.ActionCreate, ResourceType: "category", ResourceID: "17", OccurredOn: at, Result: audit.ResultSucceeded, CorrelationID: "existing-request"})
	require.NoError(t, err)
	return event
}

func TestLogQueryServiceCapturesCutReadsAndAuditsBeforeReturning(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	first, second, third := queryEvent(t, now), queryEvent(t, now.Add(-time.Hour)), queryEvent(t, now.Add(-2*time.Hour))
	reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
	sequence := []string{}
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("CaptureWatermark", ctx).Run(func(mock.Arguments) { sequence = append(sequence, "cut") }).Return(int64(50), nil).Once()
	reader.On("FindPage", ctx, audit.LogFilter{}, int64(50), (*audit.LogPosition)(nil), 3).Run(func(mock.Arguments) { sequence = append(sequence, "read") }).Return([]*audit.Event{first, second, third}, nil).Once()
	writer.On("Save", ctx, mock.MatchedBy(func(event *audit.Event) bool {
		return event != nil && event.OperatorID() == 23 && event.Action() == audit.ActionAccess && event.ResourceType() == "audit_log" && event.ResourceID() == "" && event.Result() == audit.ResultPrepared && event.CorrelationID() == "request-audit" && event.OccurredOn().Equal(now) && event.Reason() == nil
	})).Run(func(mock.Arguments) { sequence = append(sequence, "save") }).Return(nil).Once()

	page, err := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{now}).Query(ctx, "actor", "request-audit", audit.LogQuery{Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"cut", "read", "save"}, sequence)
	require.Equal(t, []*audit.Event{first, second}, page.Events)
	require.Equal(t, int64(50), page.Watermark)
	require.Equal(t, &audit.LogPosition{OccurredOn: second.OccurredOn(), ID: second.ID()}, page.Next)
	reader.AssertExpectations(t)
	writer.AssertExpectations(t)
	finder.AssertExpectations(t)
}

func TestLogQueryServiceContinuesWithoutCapturingAnotherCut(t *testing.T) {
	ctx := context.Background()
	reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
	watermark := int64(10)
	before := &audit.LogPosition{OccurredOn: time.Now().UTC(), ID: uuid.New()}
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("FindPage", ctx, audit.LogFilter{}, watermark, before, 21).Return(nil, nil).Once()
	writer.On("Save", ctx, mock.Anything).Return(nil).Once()
	page, err := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()}).Query(ctx, "actor", "request-audit", audit.LogQuery{Watermark: &watermark, Before: before})
	require.NoError(t, err)
	require.NotNil(t, page.Events)
	require.Empty(t, page.Events)
	require.Nil(t, page.Next)
	reader.AssertNotCalled(t, "CaptureWatermark", mock.Anything)
}

func TestLogQueryServiceRejectsInvalidQueryBeforeIO(t *testing.T) {
	reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
	service := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()})
	invalidOperator, negativeWatermark := 0, int64(-1)
	for _, query := range []audit.LogQuery{
		{Filter: audit.LogFilter{OperatorID: &invalidOperator}}, {Limit: 101}, {Limit: -1},
		{Watermark: &negativeWatermark, Before: &audit.LogPosition{OccurredOn: time.Now(), ID: uuid.New()}},
		{Before: &audit.LogPosition{OccurredOn: time.Now(), ID: uuid.New()}},
		{Watermark: new(int64), Before: &audit.LogPosition{ID: uuid.New()}},
	} {
		page, err := service.Query(context.Background(), "actor", "request-audit", query)
		require.ErrorIs(t, err, audit.ErrInvalidQuery)
		require.Nil(t, page.Events)
	}
	finder.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
	reader.AssertNotCalled(t, "CaptureWatermark", mock.Anything)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestLogQueryServiceFailsClosedOnReadAndWriteErrors(t *testing.T) {
	ctx := context.Background()
	readErr, writeErr := errors.New("read failed"), errors.New("write failed")
	for _, tc := range []struct {
		name              string
		readErr, writeErr error
	}{
		{name: "read", readErr: readErr}, {name: "write", writeErr: writeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
			finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
			reader.On("CaptureWatermark", ctx).Return(int64(3), nil).Once()
			reader.On("FindPage", ctx, audit.LogFilter{}, int64(3), (*audit.LogPosition)(nil), 21).Return([]*audit.Event{queryEvent(t, time.Now())}, tc.readErr).Once()
			if tc.readErr == nil {
				writer.On("Save", ctx, mock.Anything).Return(tc.writeErr).Once()
			}
			page, err := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()}).Query(ctx, "actor", "request-audit", audit.LogQuery{})
			require.Nil(t, page.Events)
			if tc.readErr != nil {
				require.ErrorIs(t, err, tc.readErr)
				writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
			} else {
				require.ErrorIs(t, err, tc.writeErr)
			}
		})
	}
}

func TestLogQueryServiceDoesNotReadWhenCutFails(t *testing.T) {
	ctx := context.Background()
	reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
	failure := errors.New("cut unavailable")
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("CaptureWatermark", ctx).Return(int64(0), failure).Once()
	page, err := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()}).Query(ctx, "actor", "request-audit", audit.LogQuery{})
	require.Nil(t, page.Events)
	require.ErrorIs(t, err, failure)
	reader.AssertNotCalled(t, "FindPage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestLogQueryServiceDoesNotReadWhenOperatorResolutionFails(t *testing.T) {
	ctx := context.Background()
	reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
	failure := errors.New("operator lookup failed")
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(0, failure).Once()
	page, err := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()}).Query(ctx, "actor", "request-audit", audit.LogQuery{})
	require.Nil(t, page.Events)
	require.ErrorIs(t, err, failure)
	reader.AssertNotCalled(t, "CaptureWatermark", mock.Anything)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestLogQueryServiceDoesNotPersistInvalidAccessEvidence(t *testing.T) {
	ctx := context.Background()
	reader, writer, finder := &logReaderMock{}, &auditWriterMock{}, &auditOperatorIDFinderMock{}
	finder.On("FindOperatorIDByAuthID", ctx, "actor").Return(23, nil).Once()
	reader.On("CaptureWatermark", ctx).Return(int64(3), nil).Once()
	reader.On("FindPage", ctx, audit.LogFilter{}, int64(3), (*audit.LogPosition)(nil), 21).Return(nil, nil).Once()
	page, err := audit.NewLogQueryService(reader, writer, finder, auditFixedClock{time.Now()}).Query(ctx, "actor", "invalid correlation with spaces", audit.LogQuery{})
	require.Nil(t, page.Events)
	require.ErrorIs(t, err, audit.ErrInvalidEvent)
	writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}
