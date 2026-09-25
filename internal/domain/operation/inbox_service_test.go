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

func inboxOperation(kind readmodel.Kind, resourceID int, startedOn time.Time) readmodel.OperationSummary {
	return readmodel.OperationSummary{
		ID:        readmodel.ID{Kind: kind, ResourceID: resourceID},
		StartedOn: startedOn,
		Stage:     readmodel.StageRequestPending,
	}
}

func TestInboxServiceReadsOneLookaheadOperationAndExposesNextPosition(t *testing.T) {
	ctx := context.Background()
	startedOn := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	first := inboxOperation(readmodel.KindJobRequest, 3, startedOn)
	second := inboxOperation(readmodel.KindServiceProposal, 9, startedOn.Add(-time.Hour))
	third := inboxOperation(readmodel.KindJobRequest, 1, startedOn.Add(-2*time.Hour))
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, (*operation.InboxPosition)(nil), 3).Return([]readmodel.OperationSummary{first, second, third}, nil).Once()

	page, err := operation.NewInboxService(reader).Query(ctx, operation.InboxQuery{Limit: 2})

	require.NoError(t, err)
	require.Equal(t, []readmodel.OperationSummary{first, second}, page.Operations)
	require.Equal(t, &operation.InboxPosition{StartedOn: second.StartedOn, ID: second.ID}, page.Next)
	reader.AssertExpectations(t)
}

func TestInboxServiceUsesDefaultLimitAndReturnsNonNilEmptyPage(t *testing.T) {
	ctx := context.Background()
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, (*operation.InboxPosition)(nil), operation.DefaultInboxLimit+1).Return(nil, nil).Once()

	page, err := operation.NewInboxService(reader).Query(ctx, operation.InboxQuery{})

	require.NoError(t, err)
	require.NotNil(t, page.Operations)
	require.Empty(t, page.Operations)
	require.Nil(t, page.Next)
}

func TestInboxServiceRejectsLimitAboveMaximumWithoutReading(t *testing.T) {
	reader := &inboxReaderMock{}

	_, err := operation.NewInboxService(reader).Query(context.Background(), operation.InboxQuery{Limit: operation.MaxInboxLimit + 1})

	require.ErrorIs(t, err, operation.ErrInvalidInboxQuery)
	reader.AssertNotCalled(t, "FindPage")
}

func TestInboxServiceWrapsReaderFailures(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("database unavailable")
	reader := &inboxReaderMock{}
	reader.On("FindPage", ctx, (*operation.InboxPosition)(nil), operation.DefaultInboxLimit+1).Return(nil, failure).Once()

	_, err := operation.NewInboxService(reader).Query(ctx, operation.InboxQuery{})

	require.ErrorIs(t, err, failure)
}
