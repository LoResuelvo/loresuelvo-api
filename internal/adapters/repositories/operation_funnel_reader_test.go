package repositories_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/require"
)

func funnelPeriod(base time.Time) operation.FunnelCriteria {
	return operation.FunnelCriteria{Period: operation.TimeWindow{From: base, To: base.Add(24 * time.Hour)}}
}
func funnelRead(t *testing.T, fixture operationInboxFixture, criteria operation.FunnelCriteria) readmodel.FunnelSnapshot {
	t.Helper()
	got, err := repositories.NewOperationFunnelReader(fixture.testContext.database).Read(t.Context(), criteria)
	require.NoError(t, err)
	return got
}
func funnelSum(t *testing.T, total readmodel.FunnelDelayAggregate, observations int64, sum string) {
	t.Helper()
	require.Equal(t, observations, total.Observations)
	got, ok := new(big.Rat).SetString(total.TotalMicroseconds)
	require.True(t, ok)
	want, ok := new(big.Rat).SetString(sum)
	require.True(t, ok)
	require.Zero(t, got.Cmp(want))
}
func funnelProvider(t *testing.T, f operationInboxFixture, index int, category string) int {
	t.Helper()
	return savedProviderIDWithData(t, f.testContext, fmt.Sprintf("auth0|funnel-%d", index), fmt.Sprintf("funnel-%d@example.com", index), "Provider", "Funnel", category)
}
func funnelCategory(t *testing.T, f operationInboxFixture, providerID int) int {
	t.Helper()
	var id int
	require.NoError(t, f.testContext.database.QueryRow(`SELECT category_id FROM providers WHERE user_id=$1`, providerID).Scan(&id))
	return id
}
func funnelAssessmentConversation(t *testing.T, f operationInboxFixture, consumerID int, base time.Time) (int, int) {
	t.Helper()
	var conversationID, messageID int
	require.NoError(t, f.testContext.database.QueryRow(`INSERT INTO conversations(type,status,created_on,updated_on) VALUES('chatbot','active',$1,$1) RETURNING id`, base).Scan(&conversationID))
	_, err := f.testContext.database.Exec(`INSERT INTO chatbot_conversations(conversation_id,consumer_id,title,context_summary,last_summarized_message_id,last_response_status) VALUES($1,$2,'Funnel','',0,'answered')`, conversationID, consumerID)
	require.NoError(t, err)
	require.NoError(t, f.testContext.database.QueryRow(`INSERT INTO messages(conversation_id,sender_role,content,created_on) VALUES($1,'consumer','Evidence',$2) RETURNING id`, conversationID, base).Scan(&messageID))
	return conversationID, messageID
}
func funnelAssessment(t *testing.T, f operationInboxFixture, conversationID, messageID, version int, categoryID *int, outcome string, base time.Time) int {
	t.Helper()
	var id int
	title, description := "Problem", "Description"
	if outcome == "collecting_information" {
		title, description = "", ""
	}
	require.NoError(t, f.testContext.database.QueryRow(`INSERT INTO problem_assessments(chatbot_conversation_id,version,outcome,problem_category_id,problem_title,problem_description,based_on_message_id,created_on) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, conversationID, version, outcome, categoryID, title, description, messageID, base).Scan(&id))
	return id
}
func funnelSource(t *testing.T, f operationInboxFixture, requestID, assessmentID int) {
	t.Helper()
	_, err := f.testContext.database.Exec(`UPDATE job_requests SET source_assessment_id=$1 WHERE id=$2`, assessmentID, requestID)
	require.NoError(t, err)
}
func funnelReport(t *testing.T, f operationInboxFixture, orderID int, at time.Time) {
	t.Helper()
	_, err := f.testContext.database.Exec(`INSERT INTO work_order_completion_reports(work_order_id,description,reported_on) VALUES($1,'Completed',$2)`, orderID, at)
	require.NoError(t, err)
}
func funnelPaid(t *testing.T, f operationInboxFixture, orderID int, at time.Time) {
	t.Helper()
	_, err := f.testContext.database.Exec(`UPDATE work_orders SET paid_on=$1,status='paid' WHERE id=$2`, at, orderID)
	require.NoError(t, err)
}
func funnelReview(t *testing.T, f operationInboxFixture, orderID int) {
	t.Helper()
	_, err := f.testContext.database.Exec(`INSERT INTO work_order_reviews(work_order_id,rating) VALUES($1,5)`, orderID)
	require.NoError(t, err)
}

func TestOperationFunnelReaderReturnsBothEmptyCohorts(t *testing.T) {
	f := newOperationInboxFixture(t)
	got := funnelRead(t, f, funnelPeriod(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)))
	require.Equal(t, readmodel.FunnelCounts{}, got.AI.Counts)
	require.Equal(t, readmodel.FunnelCounts{}, got.Manual.Counts)
	for _, c := range []readmodel.FunnelCohortSnapshot{got.AI, got.Manual} {
		for _, d := range []readmodel.FunnelDelayAggregate{c.Delays.RequestToFirstProposal, c.Delays.ProposalToConfirmedHiring, c.Delays.ConfirmedHiringToReportedCompletion, c.Delays.ReportedCompletionToFullPayment} {
			funnelSum(t, d, 0, "0")
		}
	}
}
func TestOperationFunnelReaderKeepsAssessmentVersionsAndSourceCategory(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	consumerID := savedConsumerIDForJobRequest(t, f.testContext)
	p1, p2 := funnelProvider(t, f, 1, "Plumbing"), funnelProvider(t, f, 2, "Electricity")
	c1, c2 := funnelCategory(t, f, p1), funnelCategory(t, f, p2)
	conversationID, messageID := funnelAssessmentConversation(t, f, consumerID, base)
	a1 := funnelAssessment(t, f, conversationID, messageID, 1, &c1, "professional_required", base)
	a2 := funnelAssessment(t, f, conversationID, messageID, 2, &c2, "professional_required", base.Add(time.Hour))
	funnelAssessment(t, f, conversationID, messageID, 3, nil, "self_service", base)
	funnelAssessment(t, f, conversationID, messageID, 4, nil, "collecting_information", base)
	_, err := f.testContext.database.Exec(`UPDATE chatbot_conversations SET current_assessment_id=$1 WHERE conversation_id=$2`, a2, conversationID)
	require.NoError(t, err)
	request := f.jobRequest(t, consumerID, p2, base.Add(25*time.Hour), jobrequest.StatusPending)
	funnelSource(t, f, request.ID, a1)
	for _, tc := range []struct {
		category           *int
		origins, requested int64
	}{{nil, 2, 1}, {&c1, 1, 1}, {&c2, 1, 0}, {new(2147483000), 0, 0}} {
		criteria := funnelPeriod(base)
		criteria.CategoryID = tc.category
		got := funnelRead(t, f, criteria)
		require.Equal(t, tc.origins, got.AI.Counts.Origins)
		require.Equal(t, tc.requested, got.AI.Counts.Requested)
		require.Zero(t, got.Manual.Counts.Origins)
	}
}
func TestOperationFunnelReaderUsesCurrentProviderCategoryForManualOrigin(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	consumerID := savedConsumerIDForJobRequest(t, f.testContext)
	p1, p2 := funnelProvider(t, f, 1, "Plumbing"), funnelProvider(t, f, 2, "Electricity")
	c1, c2 := funnelCategory(t, f, p1), funnelCategory(t, f, p2)
	f.jobRequest(t, consumerID, p1, base, jobrequest.StatusPending)
	_, err := f.testContext.database.Exec(`UPDATE providers SET category_id=$1 WHERE user_id=$2`, c2, p1)
	require.NoError(t, err)
	for _, tc := range []struct {
		category int
		count    int64
	}{{c1, 0}, {c2, 1}} {
		criteria := funnelPeriod(base)
		criteria.CategoryID = &tc.category
		got := funnelRead(t, f, criteria)
		require.Equal(t, tc.count, got.Manual.Counts.Origins)
		require.Equal(t, tc.count, got.Manual.Counts.Requested)
	}
}
func TestOperationFunnelReaderCountsDistinctOriginsAndCompleteOwnBranches(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	consumerID := savedConsumerIDForJobRequest(t, f.testContext)
	p1, p2 := funnelProvider(t, f, 1, "Plumbing"), funnelProvider(t, f, 2, "Plumbing")
	categoryID := funnelCategory(t, f, p1)
	conv, msg := funnelAssessmentConversation(t, f, consumerID, base)
	a1 := funnelAssessment(t, f, conv, msg, 1, &categoryID, "professional_required", base)
	funnelAssessment(t, f, conv, msg, 2, &categoryID, "professional_required", base)
	r1 := f.jobRequest(t, consumerID, p1, base.Add(25*time.Hour), jobrequest.StatusPending)
	r2 := f.jobRequest(t, consumerID, p2, base.Add(26*time.Hour), jobrequest.StatusPending)
	funnelSource(t, f, r1.ID, a1)
	funnelSource(t, f, r2.ID, a1)
	prop1 := f.proposal(t, r1, base.Add(27*time.Hour), serviceproposal.StatusAccepted)
	prop2 := f.proposal(t, r2, base.Add(28*time.Hour), serviceproposal.StatusAccepted)
	o1 := f.workOrder(t, prop1, base.Add(29*time.Hour), workorder.StatusScheduled)
	o2 := f.workOrder(t, prop2, base.Add(30*time.Hour), workorder.StatusScheduled)
	funnelReport(t, f, o1, base.Add(31*time.Hour))
	funnelPaid(t, f, o2, base.Add(32*time.Hour))
	funnelReview(t, f, o2)
	before := funnelRead(t, f, funnelPeriod(base))
	require.Equal(t, readmodel.FunnelCounts{Origins: 2, Requested: 1, Proposed: 1, Hired: 1, Completed: 1}, before.AI.Counts)
	// Report and payment on separate orders cannot form a paid/reviewed branch.
	funnelPaid(t, f, o1, base.Add(33*time.Hour))
	funnelReview(t, f, o1)
	after := funnelRead(t, f, funnelPeriod(base))
	require.Equal(t, readmodel.FunnelCounts{Origins: 2, Requested: 1, Proposed: 1, Hired: 1, Completed: 1, Paid: 1, Reviewed: 1}, after.AI.Counts)
	funnelSum(t, after.AI.Delays.RequestToFirstProposal, 2, "14400000000")
	funnelSum(t, after.AI.Delays.ProposalToConfirmedHiring, 2, "14400000000")
}
func TestOperationFunnelReaderSamplesRequestsAndOrdersWithoutJoinInflation(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	consumerID := savedConsumerIDForJobRequest(t, f.testContext)
	p1, p2, p3 := funnelProvider(t, f, 1, "Plumbing"), funnelProvider(t, f, 2, "Plumbing"), funnelProvider(t, f, 3, "Plumbing")
	r1 := f.jobRequest(t, consumerID, p1, base, jobrequest.StatusPending)
	r2 := f.jobRequest(t, consumerID, p2, base.Add(10*time.Minute), jobrequest.StatusPending)
	f.jobRequest(t, consumerID, p3, base.Add(time.Hour), jobrequest.StatusPending)
	prop1 := f.proposal(t, r1, base.Add(time.Second), serviceproposal.StatusAccepted)
	prop2 := f.proposal(t, r1, base.Add(5*time.Second), serviceproposal.StatusAccepted)
	prop3 := f.proposal(t, r2, base.Add(10*time.Minute+65670*time.Millisecond), serviceproposal.StatusAccepted)
	o1 := f.workOrder(t, prop1, base.Add(13500*time.Millisecond), workorder.StatusScheduled)
	f.workOrder(t, prop2, base.Add(25*time.Second), workorder.StatusScheduled)
	o3 := f.workOrder(t, prop3, base.Add(10*time.Minute+75670*time.Millisecond), workorder.StatusScheduled)
	funnelReport(t, f, o1, base.Add(48*time.Hour+23500*time.Millisecond))
	funnelReport(t, f, o3, base.Add(48*time.Hour+10*time.Minute+105670*time.Millisecond))
	funnelPaid(t, f, o1, base.Add(48*time.Hour+63500*time.Millisecond))
	got := funnelRead(t, f, funnelPeriod(base.Add(-10*time.Hour)))
	require.Equal(t, readmodel.FunnelCounts{Origins: 3, Requested: 3, Proposed: 2, Hired: 2, Completed: 2, Paid: 1}, got.Manual.Counts)
	funnelSum(t, got.Manual.Delays.RequestToFirstProposal, 2, "66670000")
	funnelSum(t, got.Manual.Delays.ProposalToConfirmedHiring, 3, "42500000")
	funnelSum(t, got.Manual.Delays.ConfirmedHiringToReportedCompletion, 2, "345640000000")
	funnelSum(t, got.Manual.Delays.ReportedCompletionToFullPayment, 1, "40000000")
}
func TestOperationFunnelReaderExcludesOnlyInvalidIntervalsWithoutFirstProposalFallback(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	consumerID := savedConsumerIDForJobRequest(t, f.testContext)
	p1, p2 := funnelProvider(t, f, 1, "Plumbing"), funnelProvider(t, f, 2, "Plumbing")
	r1 := f.jobRequest(t, consumerID, p1, base, jobrequest.StatusPending)
	r2 := f.jobRequest(t, consumerID, p2, base.Add(30*time.Minute), jobrequest.StatusPending)
	prop1 := f.proposal(t, r1, base.Add(-time.Minute), serviceproposal.StatusAccepted)
	f.proposal(t, r1, base.Add(2*time.Minute), serviceproposal.StatusPending)
	prop2 := f.proposal(t, r2, base.Add(31*time.Minute), serviceproposal.StatusAccepted)
	o1 := f.workOrder(t, prop1, base.Add(3*time.Minute), workorder.StatusScheduled)
	o2 := f.workOrder(t, prop2, base.Add(32*time.Minute), workorder.StatusScheduled)
	funnelReport(t, f, o1, base.Add(2*time.Minute))
	funnelReport(t, f, o2, base.Add(48*time.Hour+32*time.Minute))
	funnelPaid(t, f, o2, base.Add(48*time.Hour+33*time.Minute))
	got := funnelRead(t, f, funnelPeriod(base.Add(-10*time.Hour)))
	funnelSum(t, got.Manual.Delays.RequestToFirstProposal, 1, "60000000")
	funnelSum(t, got.Manual.Delays.ProposalToConfirmedHiring, 2, "300000000")
	funnelSum(t, got.Manual.Delays.ConfirmedHiringToReportedCompletion, 1, "172800000000")
	funnelSum(t, got.Manual.Delays.ReportedCompletionToFullPayment, 1, "60000000")
}
func TestOperationFunnelReaderRoundsNanosecondOriginBounds(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	consumerID := savedConsumerIDForJobRequest(t, f.testContext)
	p1, p2 := funnelProvider(t, f, 1, "Plumbing"), funnelProvider(t, f, 2, "Plumbing")
	f.jobRequest(t, consumerID, p1, base, jobrequest.StatusPending)
	f.jobRequest(t, consumerID, p2, base.Add(time.Microsecond), jobrequest.StatusPending)
	category := funnelCategory(t, f, p1)
	conv, msg := funnelAssessmentConversation(t, f, consumerID, base)
	funnelAssessment(t, f, conv, msg, 1, &category, "professional_required", base)
	funnelAssessment(t, f, conv, msg, 2, &category, "professional_required", base.Add(time.Microsecond))
	for _, period := range []operation.TimeWindow{{From: base.Add(time.Nanosecond), To: base.Add(2 * time.Microsecond)}, {From: base.Add(-time.Microsecond), To: base.Add(time.Nanosecond)}} {
		got := funnelRead(t, f, operation.FunnelCriteria{Period: period})
		require.EqualValues(t, 1, got.AI.Counts.Origins)
		require.EqualValues(t, 1, got.Manual.Counts.Origins)
	}
}
func TestOperationFunnelReaderIncludesZeroDurationSample(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, f.testContext)
	r := f.jobRequest(t, consumerID, providerID, base, jobrequest.StatusPending)
	f.proposal(t, r, base, serviceproposal.StatusPending)
	got := funnelRead(t, f, funnelPeriod(base))
	funnelSum(t, got.Manual.Delays.RequestToFirstProposal, 1, "0")
	funnelSum(t, got.Manual.Delays.ProposalToConfirmedHiring, 0, "0")
}

func TestOperationFunnelReaderInvalidHiringIntervalKeepsCompletionAndPaymentSamples(t *testing.T) {
	f := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, f.testContext)
	request := f.jobRequest(t, consumerID, providerID, base, jobrequest.StatusPending)
	proposal := f.proposal(t, request, base.Add(3*time.Minute), serviceproposal.StatusAccepted)
	// Historical anomalous hiring must not discard later valid intervals on the order.
	order := f.workOrder(t, proposal, base.Add(2*time.Minute), workorder.StatusScheduled)
	funnelReport(t, f, order, base.Add(4*time.Minute))
	funnelPaid(t, f, order, base.Add(5*time.Minute))
	got := funnelRead(t, f, funnelPeriod(base))
	funnelSum(t, got.Manual.Delays.ProposalToConfirmedHiring, 0, "0")
	funnelSum(t, got.Manual.Delays.ConfirmedHiringToReportedCompletion, 1, "120000000")
	funnelSum(t, got.Manual.Delays.ReportedCompletionToFullPayment, 1, "60000000")
	require.Equal(t, readmodel.FunnelCounts{Origins: 1, Requested: 1, Proposed: 1, Hired: 1, Completed: 1, Paid: 1}, got.Manual.Counts)
}
