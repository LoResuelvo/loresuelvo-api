package readmodel

import (
	"time"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type Kind string

const (
	KindJobRequest      Kind = "jr"
	KindServiceProposal Kind = "sp"
)

// ID names the resource that started the operation. A job request's operation
// continues through its conversation's first proposal; later proposals start their own.
type ID struct {
	Kind       Kind
	ResourceID int
}

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

type Alert string

const (
	AlertRequestPendingOver24h Alert = "request_pending_over_24h"
	AlertBookingDeadlinePassed Alert = "booking_deadline_passed"
	AlertDelayed               Alert = "delayed"
	AlertStalled               Alert = "stalled"
)

type Limitation string

const (
	LimitationRequestAcceptanceTimeUnavailable Limitation = "request_acceptance_time_unavailable"
)

// A nil *Owner means the owner cannot be deduced; OwnerNone means nobody must act.
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

type OperationSummary struct {
	ID                    ID
	StartedOn             time.Time
	Stage                 Stage
	JobRequest            *JobRequest
	ServiceProposal       *ServiceProposal
	WorkOrder             *WorkOrder
	Consumer              Party
	Provider              Party
	Category              *Category
	Alerts                []Alert
	NextActionOwner       *Owner
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
