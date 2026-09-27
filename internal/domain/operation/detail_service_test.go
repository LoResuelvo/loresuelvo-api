package operation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

func TestParseOperationIDStrict(t *testing.T) {
	for _, raw := range []string{"jr-1", "sp-2147483647"} {
		_, err := operation.ParseOperationID(raw)
		require.NoError(t, err)
	}
	for _, raw := range []string{"", "jr", "jr-", "jr-0", "jr-01", "jr--1", "JR-1", "wo-1", "sp-2147483648", "sp-1/extra", "sp-+1"} {
		t.Run(raw, func(t *testing.T) {
			_, err := operation.ParseOperationID(raw)
			require.ErrorIs(t, err, operation.ErrInvalidOperationID)
		})
	}
}

func TestDetailServiceAuditsBeforeReleasingDetail(t *testing.T) {
	ctx := context.Background()
	id := readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 7}
	reader := &detailReaderStub{detail: &readmodel.OperationDetail{ID: id}}
	operator := &detailOperatorStub{id: 23}
	writer := &detailAuditStub{}
	service := operation.NewDetailService(reader, operator, writer, inboxFixedClock{now: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)})
	detail, err := service.Query(ctx, "jr-7", "subject", "request-1")
	require.NoError(t, err)
	require.Equal(t, id, detail.ID)
	require.Equal(t, audit.ActionAccess, writer.event.Action())
	require.Equal(t, "job_request", writer.event.ResourceType())
	require.Equal(t, "7", writer.event.ResourceID())
	require.Equal(t, 23, writer.event.OperatorID())
	require.Equal(t, audit.ResultPrepared, writer.event.Result())
	require.Equal(t, "request-1", writer.event.CorrelationID())
	writer.err = errors.New("audit unavailable")
	detail, err = service.Query(ctx, "jr-7", "subject", "request-1")
	require.Nil(t, detail)
	require.ErrorIs(t, err, writer.err)
}

func TestDetailServiceDoesNotAuditMissingOrInvalidOperations(t *testing.T) {
	reader := &detailReaderStub{}
	operator := &detailOperatorStub{id: 23}
	writer := &detailAuditStub{}
	service := operation.NewDetailService(reader, operator, writer, inboxFixedClock{now: time.Now()})
	_, err := service.Query(context.Background(), "jr-0", "subject", "request-1")
	require.ErrorIs(t, err, operation.ErrInvalidOperationID)
	require.Zero(t, reader.calls)
	_, err = service.Query(context.Background(), "jr-1", "subject", "request-1")
	require.ErrorIs(t, err, operation.ErrOperationNotFound)
	require.Zero(t, operator.calls)
	require.Nil(t, writer.event)
}

func TestDetailServiceAuditsLaterProposalByPersistedResource(t *testing.T) {
	id := readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 9}
	reader := &detailReaderStub{detail: &readmodel.OperationDetail{ID: id}}
	writer := &detailAuditStub{}
	service := operation.NewDetailService(reader, &detailOperatorStub{id: 23}, writer, inboxFixedClock{now: time.Now()})
	_, err := service.Query(context.Background(), "sp-9", "subject", "request-2")
	require.NoError(t, err)
	require.Equal(t, "service_proposal", writer.event.ResourceType())
	require.Equal(t, "9", writer.event.ResourceID())
}
