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
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type operationInboxFixture struct {
	testContext jobRequestRepositoryTestContext
}

func newOperationInboxFixture(t *testing.T) operationInboxFixture {
	t.Helper()
	return operationInboxFixture{testContext: newJobRequestRepositoryTest(t)}
}

func (fixture operationInboxFixture) jobRequest(t *testing.T, consumerID, providerID int, createdOn time.Time, status jobrequest.Status) jobrequest.JobRequest {
	t.Helper()
	request := validJobRequest(t, consumerID, providerID)
	request.CreatedOn = createdOn
	saved, err := fixture.testContext.jobRequestRepository.SaveWithConversation(request, conversationForJobRequest(t, consumerID, providerID))
	require.NoError(t, err)
	if status != jobrequest.StatusPending {
		_, err = fixture.testContext.database.Exec(`UPDATE job_requests SET status = $1 WHERE id = $2`, status, saved.ID)
		require.NoError(t, err)
	}
	return *saved
}

func (fixture operationInboxFixture) proposal(t *testing.T, request jobrequest.JobRequest, createdOn time.Time, status serviceproposal.Status) int {
	t.Helper()
	scheduledOn := createdOn.Add(72 * time.Hour)
	var id int
	err := fixture.testContext.database.QueryRow(
		`INSERT INTO service_proposals (consumer_id, provider_id, conversation_id, amount_cents, scheduled_on, description, status,
			created_on, updated_on, currency, deposit_cents, platform_fee_total_cents, platform_fee_due_now_cents,
			booking_payment_deadline, estimated_duration_minutes)
		VALUES ($1, $2, $3, 100000, $4, 'Reparación', $5, $6, $6, 'ARS', 20000, 5000, 1000, $7, 60)
		RETURNING id`,
		request.ConsumerID, request.ProviderID, request.ConversationID, scheduledOn.UTC(), status, createdOn.UTC(),
		scheduledOn.Add(-24*time.Hour).UTC(),
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func (fixture operationInboxFixture) workOrder(t *testing.T, proposalID int, acceptedOn time.Time, status workorder.Status) int {
	t.Helper()
	var id int
	err := fixture.testContext.database.QueryRow(
		`INSERT INTO work_orders (service_proposal_id, status, accepted_on, updated_on) VALUES ($1, $2, $3, $3) RETURNING id`,
		proposalID, status, acceptedOn.UTC(),
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestOperationInboxReaderGroupsResourcesIntoOperationsNewestFirst(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ana := savedConsumerIDWithData(t, fixture.testContext, "auth0|inbox-ana", "inbox.ana@example.com", "Ana", "Perez")
	juan := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-juan", "inbox.juan@example.com", "Juan", "Gomez", "Plomeria")
	pedro := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-pedro", "inbox.pedro@example.com", "Pedro", "Dib", "Electricidad")
	base := time.Date(2026, 9, 20, 13, 0, 0, 0, time.UTC)

	hired := fixture.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	firstProposal := fixture.proposal(t, hired, base.Add(time.Hour), serviceproposal.StatusAccepted)
	firstOrder := fixture.workOrder(t, firstProposal, base.Add(2*time.Hour), workorder.StatusPaid)
	laterProposal := fixture.proposal(t, hired, base.Add(3*time.Hour), serviceproposal.StatusPending)
	pending := fixture.jobRequest(t, ana, pedro, base.Add(-time.Hour), jobrequest.StatusPending)

	operations, err := repositories.NewOperationInboxReader(fixture.testContext.database).FindPage(context.Background(), nil, 10)

	require.NoError(t, err)
	require.Equal(t, []readmodel.OperationSummary{
		{
			ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: laterProposal}, StartedOn: base.Add(3 * time.Hour),
			Stage:           readmodel.StageProposalPending,
			JobRequest:      &readmodel.JobRequest{ID: hired.ID, Status: jobrequest.StatusAccepted},
			ServiceProposal: &readmodel.ServiceProposal{ID: laterProposal, Status: serviceproposal.StatusPending},
		},
		{
			ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: hired.ID}, StartedOn: base,
			Stage:           readmodel.StageWorkOrderPaid,
			JobRequest:      &readmodel.JobRequest{ID: hired.ID, Status: jobrequest.StatusAccepted},
			ServiceProposal: &readmodel.ServiceProposal{ID: firstProposal, Status: serviceproposal.StatusAccepted},
			WorkOrder:       &readmodel.WorkOrder{ID: firstOrder, Status: workorder.StatusPaid},
		},
		{
			ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: pending.ID}, StartedOn: base.Add(-time.Hour),
			Stage:      readmodel.StageRequestPending,
			JobRequest: &readmodel.JobRequest{ID: pending.ID, Status: jobrequest.StatusPending},
		},
	}, operations)
}

func TestOperationInboxReaderContinuesAfterPositionWithDeterministicTieBreak(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	juan := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-juan", "inbox.juan@example.com", "Juan", "Gomez", "Plomeria")
	startedOn := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	ids := make([]int, 0, 3)
	for index, email := range []string{"inbox.a@example.com", "inbox.b@example.com", "inbox.c@example.com"} {
		consumerID := savedConsumerIDWithData(t, fixture.testContext, "auth0|inbox-"+email, email, "Consumer", string(rune('A'+index)))
		ids = append(ids, fixture.jobRequest(t, consumerID, juan, startedOn, jobrequest.StatusPending).ID)
	}
	reader := repositories.NewOperationInboxReader(fixture.testContext.database)

	firstPage, err := reader.FindPage(context.Background(), nil, 2)
	require.NoError(t, err)
	require.Len(t, firstPage, 2)
	last := firstPage[1]
	secondPage, err := reader.FindPage(context.Background(), &operation.InboxPosition{StartedOn: last.StartedOn, ID: last.ID}, 2)

	require.NoError(t, err)
	require.Len(t, secondPage, 1)
	assert.Equal(t, []int{ids[2], ids[1], ids[0]}, []int{firstPage[0].ID.ResourceID, firstPage[1].ID.ResourceID, secondPage[0].ID.ResourceID})
}

func TestOperationInboxReaderReturnsNonNilEmptyPage(t *testing.T) {
	fixture := newOperationInboxFixture(t)

	operations, err := repositories.NewOperationInboxReader(fixture.testContext.database).FindPage(context.Background(), nil, 10)

	require.NoError(t, err)
	assert.NotNil(t, operations)
	assert.Empty(t, operations)
}
