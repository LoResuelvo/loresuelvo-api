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
	return fixture.scheduledProposal(t, request, createdOn, createdOn.Add(72*time.Hour), 60, status)
}

func (fixture operationInboxFixture) scheduledProposal(t *testing.T, request jobrequest.JobRequest, createdOn, scheduledOn time.Time, durationMinutes int, status serviceproposal.Status) int {
	t.Helper()
	var id int
	err := fixture.testContext.database.QueryRow(
		`INSERT INTO service_proposals (consumer_id, provider_id, conversation_id, amount_cents, scheduled_on, description, status,
			created_on, updated_on, currency, deposit_cents, platform_fee_total_cents, platform_fee_due_now_cents,
			booking_payment_deadline, estimated_duration_minutes)
		VALUES ($1, $2, $3, 100000, $4, 'Reparación', $5, $6, $6, 'ARS', 20000, 5000, 1000, $7, $8)
		RETURNING id`,
		request.ConsumerID, request.ProviderID, request.ConversationID, scheduledOn.UTC(), status, createdOn.UTC(),
		scheduledOn.Add(-24*time.Hour).UTC(), durationMinutes,
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

func operationInboxCriteria(limit int) operation.InboxCriteria {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	return operation.InboxCriteria{
		Now:                  now,
		PendingRequestCutoff: now.Add(-operation.RequestResponseWindow),
		StalledCutoff:        now.Add(-operation.StallThreshold),
		Limit:                limit,
	}
}

type operationInboxRow struct {
	ID           readmodel.ID
	Stage        readmodel.Stage
	JobRequestID *int
	ProposalID   *int
	WorkOrderID  *int
	StartedOn    time.Time
}

func operationInboxRows(operations []readmodel.OperationSummary) []operationInboxRow {
	rows := make([]operationInboxRow, 0, len(operations))
	for _, found := range operations {
		row := operationInboxRow{ID: found.ID, Stage: found.Stage, StartedOn: found.StartedOn}
		if found.JobRequest != nil {
			row.JobRequestID = &found.JobRequest.ID
		}
		if found.ServiceProposal != nil {
			row.ProposalID = &found.ServiceProposal.ID
		}
		if found.WorkOrder != nil {
			row.WorkOrderID = &found.WorkOrder.ID
		}
		rows = append(rows, row)
	}
	return rows
}

func (fixture operationInboxFixture) completionReport(t *testing.T, workOrderID int, reportedOn time.Time) {
	t.Helper()
	_, err := fixture.testContext.database.Exec(
		`INSERT INTO work_order_completion_reports (work_order_id, description, reported_on) VALUES ($1, 'Trabajo terminado', $2)`,
		workOrderID, reportedOn.UTC(),
	)
	require.NoError(t, err)
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

	operations, err := repositories.NewOperationInboxReader(fixture.testContext.database).FindPage(context.Background(), operationInboxCriteria(10))

	require.NoError(t, err)
	require.Equal(t, []operationInboxRow{
		{ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: laterProposal}, Stage: readmodel.StageProposalPending,
			JobRequestID: &hired.ID, ProposalID: &laterProposal, StartedOn: base.Add(3 * time.Hour)},
		{ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: hired.ID}, Stage: readmodel.StageWorkOrderPaid,
			JobRequestID: &hired.ID, ProposalID: &firstProposal, WorkOrderID: &firstOrder, StartedOn: base},
		{ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: pending.ID}, Stage: readmodel.StageRequestPending,
			JobRequestID: &pending.ID, StartedOn: base.Add(-time.Hour)},
	}, operationInboxRows(operations))
}

func TestOperationInboxReaderSummarizesPartiesCategoryAndDates(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ana := savedConsumerIDWithData(t, fixture.testContext, "auth0|inbox-ana", "inbox.ana@example.com", "Ana", "Perez")
	juan := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-juan", "inbox.juan@example.com", "Juan", "Gomez", "Plomeria")
	base := time.Date(2026, 9, 15, 13, 0, 0, 0, time.UTC)
	request := fixture.jobRequest(t, ana, juan, base, jobrequest.StatusAccepted)
	proposalID := fixture.proposal(t, request, base.Add(24*time.Hour), serviceproposal.StatusAccepted)
	orderID := fixture.workOrder(t, proposalID, base.Add(48*time.Hour), workorder.StatusScheduled)

	operations, err := repositories.NewOperationInboxReader(fixture.testContext.database).FindPage(context.Background(), operationInboxCriteria(10))

	require.NoError(t, err)
	require.Len(t, operations, 1)
	found := operations[0]
	assert.Equal(t, readmodel.Party{ID: ana, Name: "Ana", Surname: "Perez"}, found.Consumer)
	assert.Equal(t, readmodel.Party{ID: juan, Name: "Juan", Surname: "Gomez"}, found.Provider)
	require.NotNil(t, found.Category)
	assert.Equal(t, "Plomeria", found.Category.Name)
	assert.Equal(t, &readmodel.JobRequest{ID: request.ID, Status: jobrequest.StatusAccepted, CreatedOn: base}, found.JobRequest)
	assert.Equal(t, &readmodel.ServiceProposal{
		ID: proposalID, Status: serviceproposal.StatusAccepted, CreatedOn: base.Add(24 * time.Hour),
		ScheduledOn: base.Add(96 * time.Hour), EstimatedDurationMinutes: 60, BookingPaymentDeadline: base.Add(72 * time.Hour),
	}, found.ServiceProposal)
	assert.Equal(t, &readmodel.WorkOrder{ID: orderID, Status: workorder.StatusScheduled, AcceptedOn: base.Add(48 * time.Hour)}, found.WorkOrder)
	assert.Equal(t, base.Add(48*time.Hour), *found.LastBusinessAdvanceOn)
	assert.Equal(t, []readmodel.Alert{readmodel.AlertDelayed}, found.Alerts)
}

func TestOperationInboxReaderFlagsPendingProposalsThatReachedTheirBookingDeadline(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	juan := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-juan", "inbox.juan@example.com", "Juan", "Gomez", "Plomeria")
	now := operationInboxCriteria(10).Now
	proposalsByDeadline := map[time.Duration]int{}
	for index, deadlineOffset := range []time.Duration{0, time.Minute} {
		email := fmt.Sprintf("inbox.deadline%d@example.com", index)
		consumerID := savedConsumerIDWithData(t, fixture.testContext, "auth0|"+email, email, "Consumer", "Deadline")
		request := fixture.jobRequest(t, consumerID, juan, now.Add(-96*time.Hour), jobrequest.StatusAccepted)
		proposalsByDeadline[deadlineOffset] = fixture.proposal(t, request, now.Add(deadlineOffset-48*time.Hour), serviceproposal.StatusPending)
	}

	bookedConsumerID := savedConsumerIDWithData(t, fixture.testContext, "auth0|inbox.booked@example.com", "inbox.booked@example.com", "Consumer", "Booked")
	bookedRequest := fixture.jobRequest(t, bookedConsumerID, juan, now.Add(-96*time.Hour), jobrequest.StatusAccepted)
	bookedProposal := fixture.proposal(t, bookedRequest, now.Add(-48*time.Hour), serviceproposal.StatusAccepted)
	fixture.workOrder(t, bookedProposal, now.Add(-47*time.Hour), workorder.StatusScheduled)

	operations, err := repositories.NewOperationInboxReader(fixture.testContext.database).FindPage(context.Background(), operationInboxCriteria(10))

	require.NoError(t, err)
	alertsByProposal := map[int][]readmodel.Alert{}
	for _, found := range operations {
		alertsByProposal[found.ServiceProposal.ID] = found.Alerts
	}
	assert.Equal(t, []readmodel.Alert{readmodel.AlertBookingDeadlinePassed}, alertsByProposal[proposalsByDeadline[0]])
	assert.Equal(t, []readmodel.Alert{}, alertsByProposal[proposalsByDeadline[time.Minute]])
	assert.Equal(t, []readmodel.Alert{}, alertsByProposal[bookedProposal])
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

	firstPage, err := reader.FindPage(context.Background(), operationInboxCriteria(2))
	require.NoError(t, err)
	require.Len(t, firstPage, 2)
	last := firstPage[1]
	secondPage, err := reader.FindPage(context.Background(), operation.InboxCriteria{Now: operationInboxCriteria(2).Now, After: &operation.InboxPosition{StartedOn: last.StartedOn, ID: last.ID}, Limit: 2})

	require.NoError(t, err)
	require.Len(t, secondPage, 1)
	assert.Equal(t, []int{ids[2], ids[1], ids[0]}, []int{firstPage[0].ID.ResourceID, firstPage[1].ID.ResourceID, secondPage[0].ID.ResourceID})
}

func TestOperationInboxReaderReturnsNonNilEmptyPage(t *testing.T) {
	fixture := newOperationInboxFixture(t)

	operations, err := repositories.NewOperationInboxReader(fixture.testContext.database).FindPage(context.Background(), operationInboxCriteria(10))

	require.NoError(t, err)
	assert.NotNil(t, operations)
	assert.Empty(t, operations)
}

func TestOperationInboxReaderDerivesAlertsAndLastBusinessAdvanceAtTheirExactBoundaries(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	juan := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-juan", "inbox.juan@example.com", "Juan", "Gomez", "Plomeria")
	criteria := operationInboxCriteria(20)
	now := criteria.Now
	consumers := 0
	request := func(createdOn time.Time, status jobrequest.Status) jobrequest.JobRequest {
		consumers++
		email := fmt.Sprintf("inbox.alert%d@example.com", consumers)
		consumerID := savedConsumerIDWithData(t, fixture.testContext, "auth0|"+email, email, "Consumer", "Alert")
		return fixture.jobRequest(t, consumerID, juan, createdOn, status)
	}
	unansweredForExactly24h := request(now.Add(-24*time.Hour), jobrequest.StatusPending)
	unansweredForMore := request(now.Add(-24*time.Hour-time.Second), jobrequest.StatusPending)
	acceptedWithoutProposals := request(now.Add(-240*time.Hour), jobrequest.StatusAccepted)
	endingNow := request(now.Add(-240*time.Hour), jobrequest.StatusAccepted)
	endingNowOrder := fixture.workOrder(t, fixture.scheduledProposal(t, endingNow, now.Add(-200*time.Hour), now.Add(-2*time.Hour), 120, serviceproposal.StatusAccepted), now.Add(-100*time.Hour), workorder.StatusScheduled)
	endedMinuteAgo := request(now.Add(-240*time.Hour), jobrequest.StatusAccepted)
	fixture.workOrder(t, fixture.scheduledProposal(t, endedMinuteAgo, now.Add(-200*time.Hour), now.Add(-2*time.Hour), 119, serviceproposal.StatusAccepted), now.Add(-100*time.Hour), workorder.StatusScheduled)
	awaitingBalance := request(now.Add(-240*time.Hour), jobrequest.StatusAccepted)
	awaitingOrder := fixture.workOrder(t, fixture.scheduledProposal(t, awaitingBalance, now.Add(-200*time.Hour), now.Add(-100*time.Hour), 60, serviceproposal.StatusAccepted), now.Add(-150*time.Hour), workorder.StatusAwaitingPayment)
	fixture.completionReport(t, awaitingOrder, now.Add(-73*time.Hour))
	idleProposal := request(now.Add(-240*time.Hour), jobrequest.StatusAccepted)
	fixture.proposal(t, idleProposal, now.Add(-72*time.Hour-time.Minute), serviceproposal.StatusPending)

	reader := repositories.NewOperationInboxReader(fixture.testContext.database)
	operations, err := reader.FindPage(context.Background(), criteria)

	require.NoError(t, err)
	byRequest := map[int]readmodel.OperationSummary{}
	for _, found := range operations {
		byRequest[found.JobRequest.ID] = found
	}
	assert.Equal(t, []readmodel.Alert{}, byRequest[unansweredForExactly24h.ID].Alerts)
	assert.Equal(t, now.Add(-24*time.Hour), *byRequest[unansweredForExactly24h.ID].LastBusinessAdvanceOn)
	assert.Equal(t, []readmodel.Alert{readmodel.AlertRequestPendingOver24h}, byRequest[unansweredForMore.ID].Alerts)
	assert.Equal(t, []readmodel.Alert{}, byRequest[acceptedWithoutProposals.ID].Alerts)
	assert.Nil(t, byRequest[acceptedWithoutProposals.ID].LastBusinessAdvanceOn)
	assert.Equal(t, []readmodel.Alert{}, byRequest[endingNow.ID].Alerts)
	assert.Equal(t, endingNowOrder, byRequest[endingNow.ID].WorkOrder.ID)
	assert.Equal(t, []readmodel.Alert{readmodel.AlertDelayed}, byRequest[endedMinuteAgo.ID].Alerts)
	assert.Equal(t, []readmodel.Alert{readmodel.AlertStalled}, byRequest[awaitingBalance.ID].Alerts)
	assert.Equal(t, now.Add(-73*time.Hour), *byRequest[awaitingBalance.ID].LastBusinessAdvanceOn)
	assert.Equal(t, []readmodel.Alert{readmodel.AlertBookingDeadlinePassed, readmodel.AlertStalled}, byRequest[idleProposal.ID].Alerts)

	stalled := readmodel.AlertStalled
	criteria.Filter = operation.InboxFilter{Alert: &stalled}
	filtered, err := reader.FindPage(context.Background(), criteria)
	require.NoError(t, err)
	filteredRequests := []int{}
	for _, found := range filtered {
		filteredRequests = append(filteredRequests, found.JobRequest.ID)
	}
	assert.ElementsMatch(t, []int{awaitingBalance.ID, idleProposal.ID}, filteredRequests)
}

func TestOperationInboxReaderAppliesEveryFilterWithAndSemantics(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ana := savedConsumerIDWithData(t, fixture.testContext, "auth0|inbox-ana", "inbox.ana@example.com", "Ana", "Perez")
	carla := savedConsumerIDWithData(t, fixture.testContext, "auth0|inbox-carla", "inbox.carla@example.com", "Carla", "Gomez")
	juan := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-juan", "inbox.juan@example.com", "Juan", "Gomez", "Plomeria")
	pedro := savedProviderIDWithData(t, fixture.testContext, "auth0|inbox-pedro", "inbox.pedro@example.com", "Pedro", "Dib", "Electricidad")
	criteria := operationInboxCriteria(20)
	dayStart := time.Date(2026, 9, 20, 3, 0, 0, 0, time.UTC)

	anaJuan := fixture.jobRequest(t, ana, juan, dayStart, jobrequest.StatusAccepted)
	fixture.workOrder(t, fixture.scheduledProposal(t, anaJuan, dayStart.Add(time.Hour), dayStart.Add(48*time.Hour), 60, serviceproposal.StatusAccepted), dayStart.Add(2*time.Hour), workorder.StatusScheduled)
	anaPedro := fixture.jobRequest(t, ana, pedro, dayStart.Add(24*time.Hour), jobrequest.StatusPending)
	carlaJuan := fixture.jobRequest(t, carla, juan, dayStart.Add(-time.Second), jobrequest.StatusPending)
	reader := repositories.NewOperationInboxReader(fixture.testContext.database)
	requestIDs := func(filter operation.InboxFilter, scheduled ...time.Time) []int {
		t.Helper()
		query := criteria
		query.Filter = filter
		if len(scheduled) == 2 {
			query.ScheduledWindow = &operation.TimeWindow{From: scheduled[0], To: scheduled[1]}
		}
		operations, err := reader.FindPage(context.Background(), query)
		require.NoError(t, err)
		ids := []int{}
		for _, found := range operations {
			ids = append(ids, found.JobRequest.ID)
		}
		return ids
	}
	unfiltered, err := reader.FindPage(context.Background(), criteria)
	require.NoError(t, err)
	var plumbingID int
	for _, found := range unfiltered {
		if found.Provider.ID == juan {
			plumbingID = found.Category.ID
		}
	}
	require.NotZero(t, plumbingID)
	stage := readmodel.StageRequestPending
	windowEnd := dayStart.Add(24 * time.Hour)

	assert.ElementsMatch(t, []int{anaJuan.ID, anaPedro.ID}, requestIDs(operation.InboxFilter{ConsumerID: &ana}))
	assert.ElementsMatch(t, []int{anaJuan.ID, carlaJuan.ID}, requestIDs(operation.InboxFilter{ProviderID: &juan}))
	assert.ElementsMatch(t, []int{anaJuan.ID, carlaJuan.ID}, requestIDs(operation.InboxFilter{CategoryID: &plumbingID}))
	assert.ElementsMatch(t, []int{anaJuan.ID}, requestIDs(operation.InboxFilter{StartedFrom: &dayStart, StartedTo: &windowEnd}))
	assert.ElementsMatch(t, []int{anaPedro.ID, carlaJuan.ID}, requestIDs(operation.InboxFilter{Stage: &stage}))
	assert.ElementsMatch(t, []int{anaPedro.ID}, requestIDs(operation.InboxFilter{ConsumerID: &ana, Stage: &stage}))
	scheduledDay := dayStart.Add(48 * time.Hour)
	assert.ElementsMatch(t, []int{anaJuan.ID}, requestIDs(operation.InboxFilter{}, scheduledDay, scheduledDay.Add(24*time.Hour)))
	assert.Empty(t, requestIDs(operation.InboxFilter{}, scheduledDay.Add(time.Second), scheduledDay.Add(24*time.Hour)))
}
