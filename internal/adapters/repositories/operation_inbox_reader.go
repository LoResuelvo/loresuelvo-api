package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

// operationInboxSQL groups hiring resources into operations: a job request
// continues through the first proposal of its conversation, and every other
// proposal starts its own operation. Work orders and completion reports are
// 1:1 with their parents, so no join multiplies an operation. $1 is the
// evaluation instant of derived alerts.
const operationInboxSQL = `WITH first_proposals AS (
	SELECT DISTINCT ON (conversation_id) conversation_id, id
	FROM service_proposals
	ORDER BY conversation_id, id
),
operations AS (
	SELECT 'jr'::text AS kind, jr.id AS resource_id, jr.created_on AS started_on,
		jr.id AS job_request_id, fp.id AS service_proposal_id
	FROM job_requests jr
	LEFT JOIN first_proposals fp ON fp.conversation_id = jr.conversation_id
	UNION ALL
	SELECT 'sp'::text, sp.id, sp.created_on, jr.id, sp.id
	FROM service_proposals sp
	LEFT JOIN job_requests jr ON jr.conversation_id = sp.conversation_id
	LEFT JOIN first_proposals fp ON fp.conversation_id = sp.conversation_id
	WHERE jr.id IS NULL OR fp.id <> sp.id
)
SELECT o.kind, o.resource_id, o.started_on,
	CASE
		WHEN wo.id IS NOT NULL THEN 'work_order_' || wo.status
		WHEN sp.id IS NOT NULL THEN 'proposal_' || sp.status
		ELSE 'request_' || jr.status
	END,
	jr.id, jr.status, jr.created_on,
	sp.id, sp.status, sp.created_on, sp.scheduled_on, sp.estimated_duration_minutes, sp.booking_payment_deadline,
	wo.id, wo.status, wo.accepted_on, report.reported_on, wo.paid_on,
	consumer_user.id, consumer_user.name, consumer_user.surname,
	provider_user.id, provider_user.name, provider_user.surname,
	categories.id, categories.name,
	(wo.id IS NULL AND sp.status = 'pending' AND sp.booking_payment_deadline <= $1::timestamp)
FROM operations o
LEFT JOIN job_requests jr ON jr.id = o.job_request_id
LEFT JOIN service_proposals sp ON sp.id = o.service_proposal_id
LEFT JOIN work_orders wo ON wo.service_proposal_id = sp.id
LEFT JOIN work_order_completion_reports report ON report.work_order_id = wo.id
INNER JOIN users consumer_user ON consumer_user.id = COALESCE(sp.consumer_id, jr.consumer_id)
INNER JOIN users provider_user ON provider_user.id = COALESCE(sp.provider_id, jr.provider_id)
INNER JOIN providers ON providers.user_id = provider_user.id
LEFT JOIN categories ON categories.id = providers.category_id
WHERE ($2::timestamp IS NULL
	OR (o.started_on, o.kind, o.resource_id) < ($2::timestamp, $3::text, $4::integer))
ORDER BY o.started_on DESC, o.kind DESC, o.resource_id DESC
LIMIT $5`

type OperationInboxReader struct {
	db *sql.DB
}

func NewOperationInboxReader(database *sql.DB) *OperationInboxReader {
	return &OperationInboxReader{db: database}
}

func (reader *OperationInboxReader) FindPage(ctx context.Context, criteria operation.InboxCriteria) ([]readmodel.OperationSummary, error) {
	var afterStartedOn, afterKind, afterResourceID any
	if criteria.After != nil {
		afterStartedOn = criteria.After.StartedOn.UTC()
		afterKind = string(criteria.After.ID.Kind)
		afterResourceID = criteria.After.ID.ResourceID
	}
	rows, err := reader.db.QueryContext(ctx, operationInboxSQL,
		criteria.Now.UTC(), afterStartedOn, afterKind, afterResourceID, criteria.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying operations inbox: %w", err)
	}
	defer rows.Close()

	operations := make([]readmodel.OperationSummary, 0)
	for rows.Next() {
		found, err := scanOperationSummary(rows)
		if err != nil {
			return nil, err
		}
		operations = append(operations, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating operations inbox: %w", err)
	}
	return operations, nil
}

func scanOperationSummary(rows *sql.Rows) (readmodel.OperationSummary, error) {
	var found readmodel.OperationSummary
	var kind, stage string
	var jobRequestID, proposalID, proposalDuration, workOrderID, categoryID sql.NullInt64
	var jobRequestStatus, proposalStatus, workOrderStatus, categoryName sql.NullString
	var jobRequestCreatedOn, proposalCreatedOn, proposalScheduledOn, bookingDeadline sql.NullTime
	var acceptedOn, reportedOn, paidOn sql.NullTime
	var bookingDeadlinePassed sql.NullBool
	if err := rows.Scan(
		&kind, &found.ID.ResourceID, &found.StartedOn, &stage,
		&jobRequestID, &jobRequestStatus, &jobRequestCreatedOn,
		&proposalID, &proposalStatus, &proposalCreatedOn, &proposalScheduledOn, &proposalDuration, &bookingDeadline,
		&workOrderID, &workOrderStatus, &acceptedOn, &reportedOn, &paidOn,
		&found.Consumer.ID, &found.Consumer.Name, &found.Consumer.Surname,
		&found.Provider.ID, &found.Provider.Name, &found.Provider.Surname,
		&categoryID, &categoryName,
		&bookingDeadlinePassed,
	); err != nil {
		return readmodel.OperationSummary{}, fmt.Errorf("scanning operations inbox: %w", err)
	}
	found.ID.Kind = readmodel.Kind(kind)
	found.StartedOn = found.StartedOn.UTC()
	found.Stage = readmodel.Stage(stage)
	if jobRequestID.Valid {
		found.JobRequest = &readmodel.JobRequest{
			ID: int(jobRequestID.Int64), Status: jobrequest.Status(jobRequestStatus.String),
			CreatedOn: jobRequestCreatedOn.Time.UTC(),
		}
	}
	if proposalID.Valid {
		found.ServiceProposal = &readmodel.ServiceProposal{
			ID: int(proposalID.Int64), Status: serviceproposal.Status(proposalStatus.String),
			CreatedOn: proposalCreatedOn.Time.UTC(), ScheduledOn: proposalScheduledOn.Time.UTC(),
			EstimatedDurationMinutes: int(proposalDuration.Int64), BookingPaymentDeadline: bookingDeadline.Time.UTC(),
		}
	}
	if workOrderID.Valid {
		found.WorkOrder = &readmodel.WorkOrder{
			ID: int(workOrderID.Int64), Status: workorder.Status(workOrderStatus.String),
			AcceptedOn: acceptedOn.Time.UTC(), CompletionReportedOn: optionalUTC(reportedOn), BalancePaidOn: optionalUTC(paidOn),
		}
	}
	if categoryID.Valid {
		found.Category = &readmodel.Category{ID: int(categoryID.Int64), Name: categoryName.String}
	}
	found.Alerts = []readmodel.Alert{}
	if bookingDeadlinePassed.Bool {
		found.Alerts = append(found.Alerts, readmodel.AlertBookingDeadlinePassed)
	}
	return found, nil
}

func optionalUTC(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}
