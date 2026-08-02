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
	ID                      int
	ServiceProposal         ServiceProposal
	Status                  Status
	AcceptedOn              time.Time
	CompletionAuthorization *CompletionAuthorization
}

func New(serviceProposal ServiceProposal, acceptedOn time.Time) (*WorkOrder, error) {
	if serviceProposal == nil {
		return nil, fmt.Errorf("creating work order: service proposal is required")
	}

	return &WorkOrder{
		ServiceProposal: serviceProposal,
		Status:          StatusScheduled,
		AcceptedOn:      acceptedOn,
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

func (wo *WorkOrder) CompletePayment(authorization *CompletionAuthorization) error {
	if wo == nil ||
		wo.ID <= 0 ||
		wo.ServiceProposal == nil ||
		wo.Status != StatusScheduled {
		return ErrWorkOrderNotEligibleForFullPayment
	}
	if !authorization.valid() {
		return ErrInvalidCompletionAuthorization
	}
	wo.Status = StatusPaid
	wo.CompletionAuthorization = authorization
	return nil
}

func (wo *WorkOrder) ConfirmationAuthorizationFor(consumerID int) (*CompletionAuthorization, error) {
	if wo == nil || wo.ServiceProposal == nil || consumerID <= 0 || wo.ConsumerID() != consumerID {
		return nil, ErrOnlyConsumerCanViewConfirmationCode
	}
	if wo.Status != StatusPaid || wo.CompletionAuthorization == nil {
		return nil, ErrConfirmationCodeNotAvailable
	}
	if !wo.CompletionAuthorization.valid() {
		return nil, ErrInvalidCompletionAuthorization
	}
	return wo.CompletionAuthorization, nil
}
