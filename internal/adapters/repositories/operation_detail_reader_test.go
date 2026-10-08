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
	require.Nil(t, primary.WorkOrder.CompletionReport)
	require.Nil(t, primary.WorkOrder.Review)
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

func TestOperationDetailReaderHydratesOnlyConfirmedPrivateRequestImages(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusPending)
	for index, status := range []string{"confirmed", "pending"} {
		id := fmt.Sprintf("10000000-0000-4000-8000-%012d", index+1)
		insertDetailImageFile(t, fixture, id, status, "job_request_image", now)
		_, err := fixture.testContext.database.Exec(`INSERT INTO job_request_images (job_request_id, file_id, position) VALUES ($1, $2, $3)`, request.ID, id, index)
		require.NoError(t, err)
	}
	found, err := repositories.NewOperationDetailReader(fixture.testContext.database).FindByID(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Equal(t, []readmodel.PrivateImage{{ID: "10000000-0000-4000-8000-000000000001", OriginalName: "evidence.jpg", MimeType: "image/jpeg", Purpose: "job_request_image", CreatedOn: now}}, found.JobRequest.Images)
	require.Nil(t, found.WorkOrder)
}

func TestOperationDetailReaderUsesSourceAssessmentFKAndCurrentAddress(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusPending)
	var categoryID int
	require.NoError(t, fixture.testContext.database.QueryRow(`SELECT category_id FROM providers WHERE user_id = $1`, providerID).Scan(&categoryID))
	var conversationID, firstMessageID, secondMessageID, sourceID, laterID int
	require.NoError(t, fixture.testContext.database.QueryRow(`INSERT INTO conversations (type, status, created_on, updated_on) VALUES ('chatbot', 'active', $1, $1) RETURNING id`, now).Scan(&conversationID))
	_, err := fixture.testContext.database.Exec(`INSERT INTO chatbot_conversations (conversation_id, consumer_id, title, context_summary, last_summarized_message_id, last_response_status) VALUES ($1, $2, 'Assessment', '', 0, 'answered')`, conversationID, consumerID)
	require.NoError(t, err)
	require.NoError(t, fixture.testContext.database.QueryRow(`INSERT INTO messages (conversation_id, sender_role, content, created_on) VALUES ($1, 'consumer', 'First evidence', $2) RETURNING id`, conversationID, now).Scan(&firstMessageID))
	require.NoError(t, fixture.testContext.database.QueryRow(`INSERT INTO messages (conversation_id, sender_role, content, created_on) VALUES ($1, 'consumer', 'Later evidence', $2) RETURNING id`, conversationID, now.Add(time.Minute)).Scan(&secondMessageID))
	require.NoError(t, fixture.testContext.database.QueryRow(`INSERT INTO problem_assessments (chatbot_conversation_id, version, outcome, problem_category_id, problem_title, problem_description, based_on_message_id, created_on)
		VALUES ($1, 2, 'professional_required', $2, 'Source', 'Source description', $3, $4) RETURNING id`, conversationID, categoryID, firstMessageID, now).Scan(&sourceID))
	require.NoError(t, fixture.testContext.database.QueryRow(`INSERT INTO problem_assessments (chatbot_conversation_id, version, outcome, problem_category_id, problem_title, problem_description, based_on_message_id, created_on)
		VALUES ($1, 3, 'self_service', NULL, 'Later', 'Later description', $2, $3) RETURNING id`, conversationID, secondMessageID, now.Add(time.Minute)).Scan(&laterID))
	_, err = fixture.testContext.database.Exec(`UPDATE chatbot_conversations SET current_assessment_id = $1 WHERE conversation_id = $2`, laterID, conversationID)
	require.NoError(t, err)
	_, err = fixture.testContext.database.Exec(`UPDATE job_requests SET source_assessment_id = $1 WHERE id = $2`, sourceID, request.ID)
	require.NoError(t, err)
	_, err = fixture.testContext.database.Exec(`UPDATE consumer_addresses SET street = 'Current street', street_number = '42', floor = '3', unit = 'B' WHERE consumer_id = $1`, consumerID)
	require.NoError(t, err)
	found, err := repositories.NewOperationDetailReader(fixture.testContext.database).FindByID(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Equal(t, sourceID, found.SourceAssessment.ID)
	require.Equal(t, firstMessageID, found.SourceAssessment.BasedOnMessageID)
	require.Equal(t, "Source", found.SourceAssessment.Title)
	require.Equal(t, "Current street", found.Address.Street)
	require.Equal(t, "42", found.Address.StreetNumber)
	require.Equal(t, "3", *found.Address.Floor)
	require.Equal(t, "B", *found.Address.Unit)
}

func TestOperationDetailReaderHydratesPrimaryReportAndReviewWithoutSiblingEvidence(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusAccepted)
	first := fixture.proposal(t, request, now.Add(time.Hour), serviceproposal.StatusAccepted)
	sibling := fixture.proposal(t, request, now.Add(2*time.Hour), serviceproposal.StatusAccepted)
	firstOrder := fixture.workOrder(t, first, now.Add(3*time.Hour), "scheduled")
	siblingOrder := fixture.workOrder(t, sibling, now.Add(4*time.Hour), "scheduled")
	for index, orderID := range []int{firstOrder, siblingOrder} {
		var reportID int
		err := fixture.testContext.database.QueryRow(`INSERT INTO work_order_completion_reports (work_order_id, description, reported_on) VALUES ($1, $2, $3) RETURNING id`, orderID, fmt.Sprintf("report %d", index), now.Add(time.Duration(index+5)*time.Hour)).Scan(&reportID)
		require.NoError(t, err)
		_, err = fixture.testContext.database.Exec(`INSERT INTO work_order_reviews (work_order_id, rating, description) VALUES ($1, $2, $3)`, orderID, index+4, fmt.Sprintf("review %d", index))
		require.NoError(t, err)
		id := fmt.Sprintf("20000000-0000-4000-8000-%012d", index+1)
		insertDetailImageFile(t, fixture, id, "confirmed", "work_order_completion_image", now)
		_, err = fixture.testContext.database.Exec(`INSERT INTO work_order_completion_images (completion_report_id, file_id, position) VALUES ($1, $2, 0)`, reportID, id)
		require.NoError(t, err)
	}
	reader := repositories.NewOperationDetailReader(fixture.testContext.database)
	primary, err := reader.FindByID(context.Background(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Equal(t, "report 0", primary.WorkOrder.CompletionReport.Description)
	require.Equal(t, now.Add(5*time.Hour), primary.WorkOrder.CompletionReport.ReportedOn)
	require.Equal(t, "20000000-0000-4000-8000-000000000001", primary.WorkOrder.CompletionReport.Images[0].ID)
	require.Equal(t, &readmodel.WorkOrderReview{Rating: 4, Description: "review 0"}, primary.WorkOrder.Review)
	siblingDetail, err := reader.FindByID(context.Background(), readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: sibling})
	require.NoError(t, err)
	require.Equal(t, "report 1", siblingDetail.WorkOrder.CompletionReport.Description)
	require.Equal(t, "20000000-0000-4000-8000-000000000002", siblingDetail.WorkOrder.CompletionReport.Images[0].ID)
}

func insertDetailImageFile(t *testing.T, fixture operationInboxFixture, id, status, purpose string, now time.Time) {
	t.Helper()
	_, err := fixture.testContext.database.Exec(`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, 'private', 'evidence.jpg', 'image/jpeg', 100, $3, 'private', $4, 'auth0|test', $5, $5)`, id, "files/"+id, status, purpose, now)
	require.NoError(t, err)
}

func TestOperationDetailReaderSuppressesHiddenOriginalAndKeepsRating(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusAccepted)
	proposal := fixture.proposal(t, request, now.Add(time.Hour), serviceproposal.StatusAccepted)
	orderID := fixture.workOrder(t, proposal, now.Add(2*time.Hour), "paid")
	_, err := fixture.testContext.database.ExecContext(t.Context(), `INSERT INTO work_order_reviews (work_order_id, rating, description, visible) VALUES ($1, 5, 'Restricted original', FALSE)`, orderID)
	require.NoError(t, err)
	detail, err := repositories.NewOperationDetailReader(fixture.testContext.database).FindByID(t.Context(), readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: request.ID})
	require.NoError(t, err)
	require.Equal(t, &readmodel.WorkOrderReview{Rating: 5, Description: ""}, detail.WorkOrder.Review)
}
