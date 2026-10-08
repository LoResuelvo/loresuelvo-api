package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// operationDetailSQL resolves only stable inbox operation identities. The first
// proposal remains in its job request operation; later proposals are independent.
const operationDetailSQL = operationSelectedSQL + `
SELECT o.started_on, COALESCE(sp.conversation_id, jr.conversation_id),
 jr.id, jr.status, jr.title, jr.description, jr.created_on,
 sp.id, sp.status, sp.description, sp.amount_cents, sp.currency, sp.created_on,
 sp.scheduled_on, sp.estimated_duration_minutes, sp.deposit_cents, sp.platform_fee_total_cents, sp.platform_fee_due_now_cents,
	 wo.id, wo.status, wo.accepted_on, report.reported_on, wo.paid_on,
	 report.id, report.description, review.rating, CASE WHEN review.visible THEN review.description ELSE '' END,
 consumer_user.id, consumer_user.name, consumer_user.surname,
 provider_user.id, provider_user.name, provider_user.surname, provider_category.id, provider_category.name,
 address.street, address.street_number, address.floor, address.unit,
 assessment.id, assessment.version, assessment.outcome, category.id, category.name,
 assessment.problem_title, assessment.problem_description, assessment.based_on_message_id, assessment.created_on
FROM selected o
LEFT JOIN job_requests jr ON jr.id = o.job_request_id
LEFT JOIN service_proposals sp ON sp.id = o.proposal_id
LEFT JOIN work_orders wo ON wo.service_proposal_id = sp.id
LEFT JOIN work_order_completion_reports report ON report.work_order_id = wo.id
LEFT JOIN work_order_reviews review ON review.work_order_id = wo.id
JOIN users consumer_user ON consumer_user.id = o.consumer_id
JOIN users provider_user ON provider_user.id = o.provider_id
JOIN providers provider ON provider.user_id = provider_user.id
LEFT JOIN categories provider_category ON provider_category.id = provider.category_id
LEFT JOIN consumer_addresses address ON address.consumer_id = o.consumer_id
LEFT JOIN problem_assessments assessment ON assessment.id = jr.source_assessment_id
LEFT JOIN categories category ON category.id = assessment.problem_category_id`

type OperationDetailReader struct{ db *sql.DB }

func NewOperationDetailReader(db *sql.DB) *OperationDetailReader {
	return &OperationDetailReader{db: db}
}

func (reader *OperationDetailReader) FindByID(ctx context.Context, id readmodel.ID) (detail *readmodel.OperationDetail, err error) {
	tx, err := reader.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning operation detail read: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rolling back operation detail read: %w", rollbackErr))
		}
	}()
	var found readmodel.OperationDetail
	found.ID = id
	var conversationID int
	var jrID, spID, orderID, reportID, rating, amount, duration, deposit, feeTotal, feeNow, assessmentID, categoryID, providerCategoryID, assessmentVersion, baseMessageID sql.NullInt64
	var jrStatus, jrTitle, jrDescription, spStatus, spDescription, currency, orderStatus sql.NullString
	var street, streetNumber, floor, unit sql.NullString
	var outcome, categoryName, providerCategoryName, assessmentTitle, assessmentDescription, reportDescription, reviewDescription sql.NullString
	var jrCreatedOn, spCreatedOn, scheduledOn, acceptedOn, reportedOn, paidOn, assessmentCreatedOn sql.NullTime
	err = tx.QueryRowContext(ctx, operationDetailSQL, string(id.Kind), id.ResourceID).Scan(
		&found.StartedOn, &conversationID,
		&jrID, &jrStatus, &jrTitle, &jrDescription, &jrCreatedOn,
		&spID, &spStatus, &spDescription, &amount, &currency, &spCreatedOn, &scheduledOn, &duration,
		&deposit, &feeTotal, &feeNow, &orderID, &orderStatus, &acceptedOn, &reportedOn, &paidOn,
		&reportID, &reportDescription, &rating, &reviewDescription,
		&found.Consumer.ID, &found.Consumer.Name, &found.Consumer.Surname,
		&found.Provider.ID, &found.Provider.Name, &found.Provider.Surname, &providerCategoryID, &providerCategoryName,
		&street, &streetNumber, &floor, &unit,
		&assessmentID, &assessmentVersion, &outcome, &categoryID, &categoryName,
		&assessmentTitle, &assessmentDescription, &baseMessageID, &assessmentCreatedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, operation.ErrOperationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying operation detail: %w", err)
	}
	found.StartedOn = found.StartedOn.UTC()
	if jrID.Valid {
		found.JobRequest = &readmodel.DetailJobRequest{ID: int(jrID.Int64), Status: jrStatus.String, Title: jrTitle.String, Description: jrDescription.String, CreatedOn: jrCreatedOn.Time.UTC(), Images: make([]readmodel.PrivateImage, 0)}
	}
	if spID.Valid {
		found.ServiceProposal = &readmodel.DetailProposal{ID: int(spID.Int64), Status: spStatus.String, Description: spDescription.String,
			AmountCents: amount.Int64, Currency: currency.String, CreatedOn: spCreatedOn.Time.UTC(), ScheduledOn: scheduledOn.Time.UTC(),
			EstimatedDurationMinutes: int(duration.Int64), DepositCents: deposit.Int64, PlatformFeeTotalCents: feeTotal.Int64, PlatformFeeDueNowCents: feeNow.Int64}
	}
	if orderID.Valid {
		found.WorkOrder = &readmodel.DetailWorkOrder{ID: int(orderID.Int64), Status: orderStatus.String, AcceptedOn: acceptedOn.Time.UTC(),
			CompletionReportedOn: nullTimePointer(reportedOn), BalancePaidOn: nullTimePointer(paidOn)}
		if reportID.Valid {
			found.WorkOrder.CompletionReport = &readmodel.CompletionReport{Description: reportDescription.String, ReportedOn: reportedOn.Time.UTC(), Images: make([]readmodel.PrivateImage, 0)}
		}
		if rating.Valid {
			found.WorkOrder.Review = &readmodel.WorkOrderReview{Rating: int(rating.Int64), Description: reviewDescription.String}
		}
	}
	if providerCategoryID.Valid {
		found.Category = &readmodel.Category{ID: int(providerCategoryID.Int64), Name: providerCategoryName.String}
	}
	if street.Valid {
		found.Address = &readmodel.CurrentConsumerAddress{Street: street.String, StreetNumber: streetNumber.String, Floor: nullStringPointer(floor), Unit: nullStringPointer(unit)}
	}
	if assessmentID.Valid {
		found.SourceAssessment = &readmodel.SourceAssessment{ID: int(assessmentID.Int64), Version: int(assessmentVersion.Int64), Outcome: outcome.String, Title: assessmentTitle.String, Description: assessmentDescription.String, BasedOnMessageID: int(baseMessageID.Int64), CreatedOn: assessmentCreatedOn.Time.UTC()}
		if categoryID.Valid {
			found.SourceAssessment.Category = &readmodel.Category{ID: int(categoryID.Int64), Name: categoryName.String}
		}
	}
	if err := reader.loadRelatedProposals(ctx, tx, conversationID, &found); err != nil {
		return nil, err
	}
	if found.JobRequest != nil {
		found.JobRequest.Images, err = reader.loadRequestImages(ctx, tx, found.JobRequest.ID)
		if err != nil {
			return nil, err
		}
	}
	if reportID.Valid {
		found.WorkOrder.CompletionReport.Images, err = reader.loadCompletionImages(ctx, tx, int(reportID.Int64))
		if err != nil {
			return nil, err
		}
	}
	if err := reader.loadPaymentMilestones(ctx, tx, &found); err != nil {
		return nil, err
	}
	found.Timeline = detailTimeline(&found)
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing operation detail read: %w", err)
	}
	committed = true
	return &found, nil
}

func (reader *OperationDetailReader) loadRequestImages(ctx context.Context, tx *sql.Tx, requestID int) ([]readmodel.PrivateImage, error) {
	rows, err := tx.QueryContext(ctx, `SELECT f.id::text, f.original_name, f.mime_type, f.purpose, f.created_on
		FROM job_request_images image JOIN files f ON f.id = image.file_id
		WHERE image.job_request_id = $1 AND f.status = 'confirmed' AND f.visibility = 'private' AND f.purpose = 'job_request_image'
		ORDER BY image.position, image.file_id`, requestID)
	if err != nil {
		return nil, fmt.Errorf("querying job request images: %w", err)
	}
	defer rows.Close()
	return scanPrivateImages(rows)
}

func (reader *OperationDetailReader) loadCompletionImages(ctx context.Context, tx *sql.Tx, reportID int) ([]readmodel.PrivateImage, error) {
	rows, err := tx.QueryContext(ctx, `SELECT f.id::text, f.original_name, f.mime_type, f.purpose, f.created_on
		FROM work_order_completion_images image JOIN files f ON f.id = image.file_id
		WHERE image.completion_report_id = $1 AND f.status = 'confirmed' AND f.visibility = 'private' AND f.purpose = 'work_order_completion_image'
		ORDER BY image.position, image.file_id`, reportID)
	if err != nil {
		return nil, fmt.Errorf("querying completion images: %w", err)
	}
	defer rows.Close()
	return scanPrivateImages(rows)
}

func scanPrivateImages(rows *sql.Rows) ([]readmodel.PrivateImage, error) {
	images := make([]readmodel.PrivateImage, 0)
	for rows.Next() {
		var image readmodel.PrivateImage
		if err := rows.Scan(&image.ID, &image.OriginalName, &image.MimeType, &image.Purpose, &image.CreatedOn); err != nil {
			return nil, fmt.Errorf("scanning private image: %w", err)
		}
		image.CreatedOn = image.CreatedOn.UTC()
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating private images: %w", err)
	}
	return images, nil
}

// Separate bounded queries avoid multiplying proposal rows by payment attempts.
func (reader *OperationDetailReader) loadRelatedProposals(ctx context.Context, tx *sql.Tx, conversationID int, found *readmodel.OperationDetail) error {
	rows, err := tx.QueryContext(ctx, `SELECT sp.id, sp.status, sp.description, sp.amount_cents, sp.currency, sp.created_on,
		sp.scheduled_on, sp.estimated_duration_minutes, sp.deposit_cents, sp.platform_fee_total_cents,
		sp.platform_fee_due_now_cents,
		(SELECT first.id FROM service_proposals first WHERE first.conversation_id = sp.conversation_id ORDER BY first.id LIMIT 1)
		FROM service_proposals sp WHERE sp.conversation_id = $1 ORDER BY sp.id`, conversationID)
	if err != nil {
		return fmt.Errorf("querying related proposals: %w", err)
	}
	defer rows.Close()
	found.RelatedProposals = make([]readmodel.RelatedProposal, 0)
	for rows.Next() {
		var proposal readmodel.DetailProposal
		var firstID int
		if err := rows.Scan(&proposal.ID, &proposal.Status, &proposal.Description, &proposal.AmountCents, &proposal.Currency,
			&proposal.CreatedOn, &proposal.ScheduledOn, &proposal.EstimatedDurationMinutes, &proposal.DepositCents,
			&proposal.PlatformFeeTotalCents, &proposal.PlatformFeeDueNowCents, &firstID); err != nil {
			return fmt.Errorf("scanning related proposal: %w", err)
		}
		proposal.CreatedOn = proposal.CreatedOn.UTC()
		proposal.ScheduledOn = proposal.ScheduledOn.UTC()
		id := readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: proposal.ID}
		if found.JobRequest != nil && proposal.ID == firstID {
			id = readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: found.JobRequest.ID}
		}
		found.RelatedProposals = append(found.RelatedProposals, readmodel.RelatedProposal{OperationID: id, Proposal: proposal})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating related proposals: %w", err)
	}
	return nil
}

func (reader *OperationDetailReader) loadPaymentMilestones(ctx context.Context, tx *sql.Tx, found *readmodel.OperationDetail) error {
	found.PaymentMilestones = make([]readmodel.PaymentMilestone, 0)
	if found.ServiceProposal == nil {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id::text, purpose, status, created_on FROM payment_intents
		WHERE service_proposal_id = $1 ORDER BY created_on, id`, found.ServiceProposal.ID)
	if err != nil {
		return fmt.Errorf("querying payment milestones: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var milestone readmodel.PaymentMilestone
		if err := rows.Scan(&milestone.ID, &milestone.Purpose, &milestone.Status, &milestone.CreatedOn); err != nil {
			return fmt.Errorf("scanning payment milestone: %w", err)
		}
		milestone.CreatedOn = milestone.CreatedOn.UTC()
		found.PaymentMilestones = append(found.PaymentMilestones, milestone)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating payment milestones: %w", err)
	}
	return nil
}

func detailTimeline(found *readmodel.OperationDetail) []readmodel.TimelineEvent {
	events := make([]readmodel.TimelineEvent, 0, 5+len(found.PaymentMilestones))
	appendEvent := func(kind, sourceType string, sourceID int, occurredOn time.Time) {
		events = append(events, readmodel.TimelineEvent{Type: kind, SourceType: sourceType, SourceID: strconv.Itoa(sourceID), OccurredOn: occurredOn.UTC()})
	}
	if found.ID.Kind == readmodel.KindJobRequest && found.JobRequest != nil {
		appendEvent("job_request_created", "job_request", found.JobRequest.ID, found.JobRequest.CreatedOn)
	}
	if proposal := found.ServiceProposal; proposal != nil {
		appendEvent("service_proposal_created", "service_proposal", proposal.ID, proposal.CreatedOn)
	}
	if order := found.WorkOrder; order != nil {
		appendEvent("work_order_accepted", "work_order", order.ID, order.AcceptedOn)
		if order.CompletionReportedOn != nil {
			appendEvent("completion_reported", "work_order", order.ID, *order.CompletionReportedOn)
		}
		if order.BalancePaidOn != nil {
			appendEvent("balance_paid", "work_order", order.ID, *order.BalancePaidOn)
		}
	}
	for _, payment := range found.PaymentMilestones {
		events = append(events, readmodel.TimelineEvent{Type: "payment_intent_created", SourceType: "payment_intent", SourceID: payment.ID, OccurredOn: payment.CreatedOn})
	}
	sort.Slice(events, func(i, j int) bool {
		if !events[i].OccurredOn.Equal(events[j].OccurredOn) {
			return events[i].OccurredOn.Before(events[j].OccurredOn)
		}
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].SourceID < events[j].SourceID
	})
	return events
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
