package workorder

import (
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

type Status string

const (
	StatusScheduled       Status = "scheduled"
	StatusAwaitingPayment Status = "awaiting_payment"
	StatusPaid            Status = "paid"
)

type ServiceProposal interface {
	ServiceProposalID() int
	ServiceProposalAmount() int64
	ServiceProposalCurrency() string
	ServiceProposalRemainingServiceBalance() int64
	ServiceProposalRemainingPlatformFee() int64
	ServiceProposalRemainingAmountDue() int64
	ServiceProposalScheduledOn() time.Time
	ServiceProposalDescription() string
	ConsumerID() int
	ProviderID() int
}

type WorkOrder struct {
	id              int
	serviceProposal ServiceProposal
	acceptedOn      time.Time
	state           state
}

func New(serviceProposal ServiceProposal, acceptedOn time.Time) (*WorkOrder, error) {
	if serviceProposal == nil {
		return nil, fmt.Errorf("creating work order: service proposal is required")
	}

	initialState := newScheduledState()
	return &WorkOrder{
		serviceProposal: serviceProposal,
		acceptedOn:      acceptedOn,
		state:           initialState,
	}, nil
}

func (wo *WorkOrder) ID() int {
	return wo.id
}

func (wo *WorkOrder) SetID(id int) {
	wo.id = id
}

func (wo *WorkOrder) Status() Status {
	if wo == nil || wo.state == nil {
		return ""
	}
	return wo.state.status()
}

func (wo *WorkOrder) CompletionReport() *CompletionReport {
	if wo == nil || wo.state == nil {
		return nil
	}
	return wo.state.completionReport()
}

func (wo *WorkOrder) PaidOn() time.Time {
	if wo == nil || wo.state == nil {
		return time.Time{}
	}
	return wo.state.paidOn()
}

func (wo *WorkOrder) Review() *Review {
	if wo == nil || wo.state == nil {
		return nil
	}
	return wo.state.review()
}

func (wo *WorkOrder) AcceptedOn() time.Time {
	return wo.acceptedOn
}

func (wo *WorkOrder) ServiceProposal() ServiceProposal {
	return wo.serviceProposal
}

func (wo *WorkOrder) ServiceProposalID() int {
	return wo.serviceProposal.ServiceProposalID()
}

func (wo *WorkOrder) Amount() int64 {
	return wo.serviceProposal.ServiceProposalAmount()
}

func (wo *WorkOrder) Currency() string {
	return wo.serviceProposal.ServiceProposalCurrency()
}

func (wo *WorkOrder) RemainingServiceBalance() int64 {
	return wo.serviceProposal.ServiceProposalRemainingServiceBalance()
}

func (wo *WorkOrder) RemainingPlatformFee() int64 {
	return wo.serviceProposal.ServiceProposalRemainingPlatformFee()
}

func (wo *WorkOrder) RemainingAmountDue() int64 {
	return wo.serviceProposal.ServiceProposalRemainingAmountDue()
}

func (wo *WorkOrder) ScheduledOn() time.Time {
	return wo.serviceProposal.ServiceProposalScheduledOn()
}

func (wo *WorkOrder) Description() string {
	return wo.serviceProposal.ServiceProposalDescription()
}

func (wo *WorkOrder) ConsumerID() int {
	return wo.serviceProposal.ConsumerID()
}

func (wo *WorkOrder) ProviderID() int {
	return wo.serviceProposal.ProviderID()
}

func (wo *WorkOrder) ReportCompletion(providerID int, report *CompletionReport) error {
	if wo == nil || wo.id <= 0 || wo.serviceProposal == nil {
		return ErrInvalidWorkOrderIdentity
	}
	if providerID <= 0 || wo.ProviderID() != providerID {
		return ErrOnlyAssignedProviderCanReport
	}
	if report == nil {
		return ErrCompletionReportRequired
	}
	if report.ReportedOn().UTC().Before(wo.ScheduledOn().UTC()) {
		return ErrWorkOrderNotReadyForCompletion
	}

	nextState, err := wo.state.reportCompletion(report)
	if err != nil {
		return err
	}
	wo.state = nextState
	return nil
}

func (wo *WorkOrder) AuthorizeBalanceCheckout(consumerID int, now time.Time) error {
	if wo == nil || wo.id <= 0 || wo.serviceProposal == nil {
		return ErrInvalidWorkOrderIdentity
	}
	if consumerID <= 0 || wo.ConsumerID() != consumerID {
		return ErrOnlyWorkOrderConsumerCanCheckout
	}
	if now.IsZero() {
		return ErrWorkOrderNotScheduledYet
	}
	if now.UTC().Before(wo.ScheduledOn().UTC()) {
		return ErrWorkOrderNotScheduledYet
	}
	return wo.state.authorizeBalanceCheckout()
}

func (wo *WorkOrder) RegisterApprovedBalancePayment(paidOn time.Time) error {
	if wo == nil || wo.id <= 0 || wo.serviceProposal == nil {
		return ErrWorkOrderNotEligibleForFullPayment
	}
	nextState, err := wo.state.registerApprovedBalancePayment(paidOn)
	if err != nil {
		return err
	}
	wo.state = nextState
	return nil
}

func (wo *WorkOrder) AddReview(reviewer *consumer.Consumer, review *Review) error {
	if wo == nil || wo.id <= 0 || wo.serviceProposal == nil {
		return ErrInvalidWorkOrderIdentity
	}
	if reviewer == nil || reviewer.ID() <= 0 || reviewer.ID() != wo.ConsumerID() {
		return ErrOnlyWorkOrderConsumerCanReview
	}
	if review == nil {
		return ErrReviewRequired
	}

	nextState, err := wo.state.addReview(review)
	if err != nil {
		return err
	}
	wo.state = nextState
	return nil
}
