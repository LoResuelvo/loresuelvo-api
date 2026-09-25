package operation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

var inboxNow = time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)

func inboxOperation(kind readmodel.Kind, resourceID int, startedOn time.Time, stage readmodel.Stage, alerts ...readmodel.Alert) readmodel.OperationSummary {
	return readmodel.OperationSummary{
		ID:        readmodel.ID{Kind: kind, ResourceID: resourceID},
		StartedOn: startedOn,
		Stage:     stage,
		Alerts:    alerts,
	}
}

func inboxService(reader operation.InboxReader) *operation.InboxService {
	return operation.NewInboxService(reader, inboxFixedClock{now: inboxNow})
}

func TestInboxServiceReadsOneLookaheadOperationAndExposesNextPosition(t *testing.T) {
	ctx := context.Background()
	first := inboxOperation(readmodel.KindJobRequest, 3, inboxNow, readmodel.StageRequestPending)
	second := inboxOperation(readmodel.KindServiceProposal, 9, inboxNow.Add(-time.Hour), readmodel.StageRequestPending)
	third := inboxOperation(readmodel.KindJobRequest, 1, inboxNow.Add(-2*time.Hour), readmodel.StageRequestPending)
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, operation.InboxCriteria{Now: inboxNow, Limit: 3}).
		Return([]readmodel.OperationSummary{first, second, third}, nil).Once()

	page, err := inboxService(reader).Query(ctx, operation.InboxQuery{Limit: 2})

	require.NoError(t, err)
	require.Len(t, page.Operations, 2)
	require.Equal(t, []readmodel.ID{first.ID, second.ID}, []readmodel.ID{page.Operations[0].ID, page.Operations[1].ID})
	require.Equal(t, &operation.InboxPosition{StartedOn: second.StartedOn, ID: second.ID}, page.Next)
	reader.AssertExpectations(t)
}

func TestInboxServiceUsesDefaultLimitAndReturnsNonNilEmptyPage(t *testing.T) {
	ctx := context.Background()
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, operation.InboxCriteria{Now: inboxNow, Limit: operation.DefaultInboxLimit + 1}).Return(nil, nil).Once()

	page, err := inboxService(reader).Query(ctx, operation.InboxQuery{})

	require.NoError(t, err)
	require.NotNil(t, page.Operations)
	require.Empty(t, page.Operations)
	require.Nil(t, page.Next)
}

func TestInboxServiceDeducesNextActionOwnerFromStage(t *testing.T) {
	consumer, provider, none := readmodel.OwnerConsumer, readmodel.OwnerProvider, readmodel.OwnerNone
	for _, tc := range []struct {
		name     string
		stage    readmodel.Stage
		alerts   []readmodel.Alert
		expected *readmodel.Owner
	}{
		{"pending request", readmodel.StageRequestPending, nil, &provider},
		{"accepted request", readmodel.StageRequestAccepted, nil, &provider},
		{"bookable proposal", readmodel.StageProposalPending, nil, &consumer},
		{"proposal past booking deadline", readmodel.StageProposalPending, []readmodel.Alert{readmodel.AlertBookingDeadlinePassed}, nil},
		{"rejected proposal", readmodel.StageProposalRejected, nil, nil},
		{"scheduled order", readmodel.StageWorkOrderScheduled, nil, &provider},
		{"order awaiting balance", readmodel.StageWorkOrderAwaitingPayment, nil, &consumer},
		{"paid order", readmodel.StageWorkOrderPaid, nil, &none},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			reader := &inboxReaderMock{}
			reader.On("FindPage", ctx, operation.InboxCriteria{Now: inboxNow, Limit: operation.DefaultInboxLimit + 1}).
				Return([]readmodel.OperationSummary{inboxOperation(readmodel.KindJobRequest, 1, inboxNow, tc.stage, tc.alerts...)}, nil).Once()

			page, err := inboxService(reader).Query(ctx, operation.InboxQuery{})

			require.NoError(t, err)
			require.Equal(t, tc.expected, page.Operations[0].NextActionOwner)
		})
	}
}

func TestInboxServiceRejectsLimitAboveMaximumWithoutReading(t *testing.T) {
	reader := &inboxReaderMock{}

	_, err := inboxService(reader).Query(context.Background(), operation.InboxQuery{Limit: operation.MaxInboxLimit + 1})

	require.ErrorIs(t, err, operation.ErrInvalidInboxQuery)
	reader.AssertNotCalled(t, "FindPage")
}

func TestInboxServiceWrapsReaderFailures(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("database unavailable")
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, operation.InboxCriteria{Now: inboxNow, Limit: operation.DefaultInboxLimit + 1}).Return(nil, failure).Once()

	_, err := inboxService(reader).Query(ctx, operation.InboxQuery{})

	require.ErrorIs(t, err, failure)
}
