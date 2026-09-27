package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/require"
)

func TestOperationDetailReaderSeparatesFirstAndLaterProposals(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusAccepted)
	first := fixture.proposal(t, request, now.Add(time.Hour), serviceproposal.StatusPending)
	later := fixture.proposal(t, request, now.Add(2*time.Hour), serviceproposal.StatusPending)
	reader := repositories.NewOperationDetailReader(fixture.testContext.database)
	primary, err := reader.FindByID(ctx, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Equal(t, request.ID, primary.JobRequest.ID)
	require.Equal(t, first, primary.ServiceProposal.ID)
	require.Nil(t, primary.SourceAssessment)
	sibling, err := reader.FindByID(ctx, readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: later})
	require.NoError(t, err)
	require.Equal(t, later, sibling.ServiceProposal.ID)
	require.Equal(t, request.ID, sibling.JobRequest.ID)
	_, err = reader.FindByID(ctx, readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: first})
	require.ErrorIs(t, err, operation.ErrOperationNotFound)
}
