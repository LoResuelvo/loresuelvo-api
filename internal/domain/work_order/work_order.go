package workorder

import (
	"fmt"
	"time"
)

type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusPaid      Status = "paid"
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
	ID              int
	ServiceProposal ServiceProposal
	Status          Status
	AcceptedOn      time.Time
	state           state
}

func New(serviceProposal ServiceProposal, acceptedOn time.Time) (*WorkOrder, error) {
	if serviceProposal == nil {
		return nil, fmt.Errorf("creating work order: service proposal is required")
	}

	initialState := newScheduledState()
	return &WorkOrder{
		ServiceProposal: serviceProposal,
		Status:          initialState.status(),
		AcceptedOn:      acceptedOn,
		state:           initialState,
	}, nil
}

func (wo *WorkOrder) ServiceProposalID() int {
	return wo.ServiceProposal.ServiceProposalID()
}

func (wo *WorkOrder) Amount() int64 {
	return wo.ServiceProposal.ServiceProposalAmount()
}

func (wo *WorkOrder) Currency() string {
	return wo.ServiceProposal.ServiceProposalCurrency()
}

func (wo *WorkOrder) RemainingServiceBalance() int64 {
	return wo.ServiceProposal.ServiceProposalRemainingServiceBalance()
}

func (wo *WorkOrder) RemainingPlatformFee() int64 {
	return wo.ServiceProposal.ServiceProposalRemainingPlatformFee()
}

func (wo *WorkOrder) RemainingAmountDue() int64 {
	return wo.ServiceProposal.ServiceProposalRemainingAmountDue()
}

func (wo *WorkOrder) ScheduledOn() time.Time {
	return wo.ServiceProposal.ServiceProposalScheduledOn()
}

func (wo *WorkOrder) Description() string {
	return wo.ServiceProposal.ServiceProposalDescription()
}

func (wo *WorkOrder) ConsumerID() int {
	return wo.ServiceProposal.ConsumerID()
}

func (wo *WorkOrder) ProviderID() int {
	return wo.ServiceProposal.ProviderID()
}

func (wo *WorkOrder) MarkPaid() error {
	if wo == nil ||
		wo.ID <= 0 ||
		wo.ServiceProposal == nil {
		return ErrWorkOrderNotEligibleForFullPayment
	}

	currentState := wo.state
	if currentState == nil || currentState.status() != wo.Status {
		currentState = stateFromStatus(wo.Status)
	}

	nextState, err := currentState.markPaid()
	if err != nil {
		return err
	}
	wo.state = nextState
	wo.Status = nextState.status()
	return nil
}
