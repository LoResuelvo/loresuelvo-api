package repositories

import (
	"context"
	"database/sql"
	"fmt"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

// operationInboxSQL groups hiring resources into operations: a job request
// continues through the first proposal of its conversation, and every other
// proposal starts its own operation. Work orders are 1:1 with proposals, so no
// join multiplies an operation.
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
	jr.id, jr.status, sp.id, sp.status, wo.id, wo.status,
	CASE
		WHEN wo.id IS NOT NULL THEN 'work_order_' || wo.status
		WHEN sp.id IS NOT NULL THEN 'proposal_' || sp.status
		ELSE 'request_' || jr.status
	END
FROM operations o
LEFT JOIN job_requests jr ON jr.id = o.job_request_id
LEFT JOIN service_proposals sp ON sp.id = o.service_proposal_id
LEFT JOIN work_orders wo ON wo.service_proposal_id = sp.id
WHERE ($1::timestamp IS NULL
	OR (o.started_on, o.kind, o.resource_id) < ($1::timestamp, $2::text, $3::integer))
ORDER BY o.started_on DESC, o.kind DESC, o.resource_id DESC
LIMIT $4`

type OperationInboxReader struct {
	db *sql.DB
}

func NewOperationInboxReader(database *sql.DB) *OperationInboxReader {
	return &OperationInboxReader{db: database}
}

func (reader *OperationInboxReader) FindPage(ctx context.Context, after *operation.InboxPosition, limit int) ([]readmodel.OperationSummary, error) {
	var afterStartedOn, afterKind, afterResourceID any
	if after != nil {
		afterStartedOn = after.StartedOn.UTC()
		afterKind = string(after.ID.Kind)
		afterResourceID = after.ID.ResourceID
	}
	rows, err := reader.db.QueryContext(ctx, operationInboxSQL, afterStartedOn, afterKind, afterResourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying operations inbox: %w", err)
	}
	defer rows.Close()

	operations := make([]readmodel.OperationSummary, 0)
	for rows.Next() {
		var found readmodel.OperationSummary
		var kind, stage string
		var jobRequestID, serviceProposalID, workOrderID sql.NullInt64
		var jobRequestStatus, serviceProposalStatus, workOrderStatus sql.NullString
		if err := rows.Scan(
			&kind, &found.ID.ResourceID, &found.StartedOn,
			&jobRequestID, &jobRequestStatus,
			&serviceProposalID, &serviceProposalStatus,
			&workOrderID, &workOrderStatus,
			&stage,
		); err != nil {
			return nil, fmt.Errorf("scanning operations inbox: %w", err)
		}
		found.ID.Kind = readmodel.Kind(kind)
		found.StartedOn = found.StartedOn.UTC()
		found.Stage = readmodel.Stage(stage)
		if jobRequestID.Valid {
			found.JobRequest = &readmodel.JobRequest{ID: int(jobRequestID.Int64), Status: jobrequest.Status(jobRequestStatus.String)}
		}
		if serviceProposalID.Valid {
			found.ServiceProposal = &readmodel.ServiceProposal{ID: int(serviceProposalID.Int64), Status: serviceproposal.Status(serviceProposalStatus.String)}
		}
		if workOrderID.Valid {
			found.WorkOrder = &readmodel.WorkOrder{ID: int(workOrderID.Int64), Status: workorder.Status(workOrderStatus.String)}
		}
		operations = append(operations, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating operations inbox: %w", err)
	}
	return operations, nil
}
