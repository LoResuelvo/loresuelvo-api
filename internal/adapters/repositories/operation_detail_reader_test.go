package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/require"
)

func TestOperationDetailReaderScopesPaymentMilestonesAndTimelineToPrimaryProposal(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ctx := context.Background()
	started := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, started, jobrequest.StatusAccepted)
	first := fixture.proposal(t, request, started.Add(time.Hour), serviceproposal.StatusAccepted)
	later := fixture.proposal(t, request, started.Add(2*time.Hour), serviceproposal.StatusAccepted)
	firstOrder := fixture.workOrder(t, first, started.Add(3*time.Hour), "scheduled")
	laterOrder := fixture.workOrder(t, later, started.Add(4*time.Hour), "scheduled")
	for i, proposal := range []int{first, later} {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		_, err := fixture.testContext.database.Exec(`INSERT INTO payment_intents
			(id, service_proposal_id, purpose, currency, seller_amount_cents, platform_fee_cents, total_amount_cents,
			status, created_on, updated_on) VALUES ($1, $2, 'booking_deposit', 'ARS', 100, 10, 110, 'rejected', $3, $3)`,
			id, proposal, started.Add(time.Duration(i+5)*time.Hour))
		require.NoError(t, err)
	}
	reader := repositories.NewOperationDetailReader(fixture.testContext.database)
	primary, err := reader.FindByID(ctx, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Len(t, primary.RelatedProposals, 2)
	require.Equal(t, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID}, primary.RelatedProposals[0].OperationID)
	require.Equal(t, readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: later}, primary.RelatedProposals[1].OperationID)
	require.Equal(t, int64(100000), primary.RelatedProposals[0].Proposal.AmountCents)
	require.Equal(t, firstOrder, primary.WorkOrder.ID)
	require.Len(t, primary.PaymentMilestones, 1)
	require.Equal(t, "00000000-0000-4000-8000-000000000001", primary.PaymentMilestones[0].ID)
	require.Equal(t, "job_request_created", primary.Timeline[0].Type)
	for _, event := range primary.Timeline {
		if event.SourceType == "service_proposal" {
			require.NotEqual(t, fmt.Sprint(later), event.SourceID)
		}
	}
	sibling, err := reader.FindByID(ctx, readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: later})
	require.NoError(t, err)
	require.Equal(t, laterOrder, sibling.WorkOrder.ID)
	require.Len(t, sibling.PaymentMilestones, 1)
	require.Equal(t, "00000000-0000-4000-8000-000000000002", sibling.PaymentMilestones[0].ID)
	for _, event := range sibling.Timeline {
		require.NotEqual(t, "job_request_created", event.Type)
		if event.SourceType == "service_proposal" {
			require.NotEqual(t, fmt.Sprint(first), event.SourceID)
		}
	}
}

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
