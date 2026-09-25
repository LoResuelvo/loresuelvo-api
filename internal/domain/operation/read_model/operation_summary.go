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

// Alert is derived from persisted state and its timestamps; it is never stored
// and never changes that state.
type Alert string

const (
	// AlertRequestPendingOver24h marks a job request still awaiting the
	// provider's answer more than 24 hours after it was created.
	AlertRequestPendingOver24h Alert = "request_pending_over_24h"
	// AlertBookingDeadlinePassed marks a pending proposal whose booking deposit
	// can no longer be paid.
	AlertBookingDeadlinePassed Alert = "booking_deadline_passed"
	// AlertDelayed marks a scheduled work order whose expected end has passed
	// without a completion report. It does not prove the provider missed it.
	AlertDelayed Alert = "delayed"
	// AlertStalled marks an operation awaiting a party whose last business
	// advance is more than 72 hours old. Messages are not business advances.
	AlertStalled Alert = "stalled"
)

// Limitation explains why a derived value could not be established.
type Limitation string

const (
	// LimitationRequestAcceptanceTimeUnavailable: job requests do not record
	// when they were accepted, so an accepted request without proposals has no
	// known last business advance.
	LimitationRequestAcceptanceTimeUnavailable Limitation = "request_acceptance_time_unavailable"
)

// Owner is who must act next. OwnerNone means the operation needs no further
// action; an owner that cannot be deduced is a nil *Owner.
type Owner string

const (
	OwnerConsumer Owner = "consumer"
	OwnerProvider Owner = "provider"
	OwnerNone     Owner = "none"
)

type JobRequest struct {
	ID        int
	Status    jobrequest.Status
	CreatedOn time.Time
}

type ServiceProposal struct {
	ID                       int
	Status                   serviceproposal.Status
	CreatedOn                time.Time
	ScheduledOn              time.Time
	EstimatedDurationMinutes int
	BookingPaymentDeadline   time.Time
}

type WorkOrder struct {
	ID                   int
	Status               workorder.Status
	AcceptedOn           time.Time
	CompletionReportedOn *time.Time
	BalancePaidOn        *time.Time
}

type Party struct {
	ID      int
	Name    string
	Surname string
}

type Category struct {
	ID   int
	Name string
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
	Consumer        Party
	Provider        Party
	Category        *Category
	Alerts          []Alert
	NextActionOwner *Owner
	// LastBusinessAdvanceOn is nil when no business timestamp establishes it.
	LastBusinessAdvanceOn *time.Time
	Limitations           []Limitation
}

func (summary OperationSummary) HasAlert(alert Alert) bool {
	for _, found := range summary.Alerts {
		if found == alert {
			return true
		}
	}
	return false
}
