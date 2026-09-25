package readmodel

import (
	"time"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

// Kind names the resource that started an operation.
type Kind string

const (
	KindJobRequest      Kind = "jr"
	KindServiceProposal Kind = "sp"
)

// ID identifies an operation by the resource that started it. A job request
// operation continues through the first proposal of its conversation and that
// proposal's work order; every later proposal starts its own operation.
type ID struct {
	Kind       Kind
	ResourceID int
}

// Stage is the persisted status of the most advanced resource of an operation.
type Stage string

const (
	StageRequestPending           Stage = "request_pending"
	StageRequestAccepted          Stage = "request_accepted"
	StageProposalPending          Stage = "proposal_pending"
	StageProposalRejected         Stage = "proposal_rejected"
	StageWorkOrderScheduled       Stage = "work_order_scheduled"
	StageWorkOrderAwaitingPayment Stage = "work_order_awaiting_payment"
	StageWorkOrderPaid            Stage = "work_order_paid"
)

type JobRequest struct {
	ID     int
	Status jobrequest.Status
}

type ServiceProposal struct {
	ID     int
	Status serviceproposal.Status
}

type WorkOrder struct {
	ID     int
	Status workorder.Status
}

// OperationSummary is the bounded administrative view of one operation.
// Nil references mean the operation has not reached that resource.
type OperationSummary struct {
	ID              ID
	StartedOn       time.Time
	Stage           Stage
	JobRequest      *JobRequest
	ServiceProposal *ServiceProposal
	WorkOrder       *WorkOrder
}
