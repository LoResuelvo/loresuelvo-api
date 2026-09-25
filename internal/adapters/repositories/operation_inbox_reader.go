package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

// Filters use the values computed once in "derived".
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
),
resources AS (
	SELECT o.kind, o.resource_id, o.started_on,
		CASE
			WHEN wo.id IS NOT NULL THEN 'work_order_' || wo.status
			WHEN sp.id IS NOT NULL THEN 'proposal_' || sp.status
			ELSE 'request_' || jr.status
		END AS stage,
		jr.id AS job_request_id, jr.status AS job_request_status, jr.created_on AS job_request_created_on,
		sp.id AS proposal_id, sp.status AS proposal_status, sp.created_on AS proposal_created_on,
		sp.scheduled_on, sp.estimated_duration_minutes, sp.booking_payment_deadline,
		wo.id AS work_order_id, wo.status AS work_order_status, wo.accepted_on, report.reported_on, wo.paid_on,
		COALESCE(sp.consumer_id, jr.consumer_id) AS consumer_id,
		COALESCE(sp.provider_id, jr.provider_id) AS provider_id
	FROM operations o
	LEFT JOIN job_requests jr ON jr.id = o.job_request_id
	LEFT JOIN service_proposals sp ON sp.id = o.service_proposal_id
	LEFT JOIN work_orders wo ON wo.service_proposal_id = sp.id
	LEFT JOIN work_order_completion_reports report ON report.work_order_id = wo.id
),
advanced AS (
	SELECT r.*,
		CASE r.stage
			WHEN 'request_pending' THEN r.job_request_created_on
			WHEN 'request_accepted' THEN NULL
			WHEN 'work_order_scheduled' THEN r.accepted_on
			WHEN 'work_order_awaiting_payment' THEN r.reported_on
			WHEN 'work_order_paid' THEN r.paid_on
			ELSE r.proposal_created_on
		END AS last_business_advance_on
	FROM resources r
),
derived AS (
	SELECT a.*, array_remove(ARRAY[
		CASE WHEN a.stage = 'request_pending' AND a.job_request_created_on < $2::timestamp
			THEN 'request_pending_over_24h' END,
		CASE WHEN a.stage = 'proposal_pending' AND a.booking_payment_deadline <= $1::timestamp
			THEN 'booking_deadline_passed' END,
		CASE WHEN a.stage = 'work_order_scheduled'
				AND a.scheduled_on + make_interval(mins => a.estimated_duration_minutes) < $1::timestamp
			THEN 'delayed' END,
		CASE WHEN a.stage IN ('request_pending', 'proposal_pending', 'work_order_awaiting_payment')
				AND a.last_business_advance_on < $3::timestamp
			THEN 'stalled' END
	], NULL) AS alerts
	FROM advanced a
)
SELECT d.kind, d.resource_id, d.started_on, d.stage,
	d.job_request_id, d.job_request_status, d.job_request_created_on,
	d.proposal_id, d.proposal_status, d.proposal_created_on, d.scheduled_on, d.estimated_duration_minutes, d.booking_payment_deadline,
	d.work_order_id, d.work_order_status, d.accepted_on, d.reported_on, d.paid_on,
	consumer_user.id, consumer_user.name, consumer_user.surname,
	provider_user.id, provider_user.name, provider_user.surname,
	categories.id, categories.name,
	array_to_string(d.alerts, ','), d.last_business_advance_on
FROM derived d
INNER JOIN users consumer_user ON consumer_user.id = d.consumer_id
INNER JOIN users provider_user ON provider_user.id = d.provider_id
INNER JOIN providers ON providers.user_id = provider_user.id
LEFT JOIN categories ON categories.id = providers.category_id
WHERE ($4::text IS NULL OR $4::text = ANY(d.alerts))
	AND ($9::integer IS NULL OR d.consumer_id = $9::integer)
	AND ($10::integer IS NULL OR d.provider_id = $10::integer)
	AND ($11::integer IS NULL OR providers.category_id = $11::integer)
	AND ($12::timestamp IS NULL OR d.started_on >= $12::timestamp)
	AND ($13::timestamp IS NULL OR d.started_on < $13::timestamp)
	AND ($14::text IS NULL OR d.stage = $14::text)
	AND ($15::timestamp IS NULL
		OR (d.work_order_id IS NOT NULL AND d.scheduled_on >= $15::timestamp AND d.scheduled_on < $16::timestamp))
	AND ($5::timestamp IS NULL
		OR (d.started_on, d.kind, d.resource_id) < ($5::timestamp, $6::text, $7::integer))
ORDER BY d.started_on DESC, d.kind DESC, d.resource_id DESC
LIMIT $8`

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
	filter := criteria.Filter
	var alert, stage any
	if filter.Alert != nil {
		alert = string(*filter.Alert)
	}
	if filter.Stage != nil {
		stage = string(*filter.Stage)
	}
	var scheduledFrom, scheduledTo any
	if window := criteria.ScheduledWindow; window != nil {
		scheduledFrom, scheduledTo = window.From.UTC(), window.To.UTC()
	}
	rows, err := reader.db.QueryContext(ctx, operationInboxSQL,
		criteria.Now.UTC(), criteria.PendingRequestCutoff.UTC(), criteria.StalledCutoff.UTC(), alert,
		afterStartedOn, afterKind, afterResourceID, criteria.Limit,
		optionalInteger(filter.ConsumerID), optionalInteger(filter.ProviderID), optionalInteger(filter.CategoryID),
		optionalInstant(filter.StartedFrom), optionalInstant(filter.StartedTo), stage,
		scheduledFrom, scheduledTo,
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
	var acceptedOn, reportedOn, paidOn, lastBusinessAdvanceOn sql.NullTime
	var alerts string
	if err := rows.Scan(
		&kind, &found.ID.ResourceID, &found.StartedOn, &stage,
		&jobRequestID, &jobRequestStatus, &jobRequestCreatedOn,
		&proposalID, &proposalStatus, &proposalCreatedOn, &proposalScheduledOn, &proposalDuration, &bookingDeadline,
		&workOrderID, &workOrderStatus, &acceptedOn, &reportedOn, &paidOn,
		&found.Consumer.ID, &found.Consumer.Name, &found.Consumer.Surname,
		&found.Provider.ID, &found.Provider.Name, &found.Provider.Surname,
		&categoryID, &categoryName,
		&alerts, &lastBusinessAdvanceOn,
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
	if alerts != "" {
		for _, alert := range strings.Split(alerts, ",") {
			found.Alerts = append(found.Alerts, readmodel.Alert(alert))
		}
	}
	found.LastBusinessAdvanceOn = optionalUTC(lastBusinessAdvanceOn)
	return found, nil
}

func optionalUTC(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

func optionalInteger(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalInstant(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
