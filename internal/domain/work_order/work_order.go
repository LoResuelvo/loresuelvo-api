package workorder

import (
	"fmt"
	"time"
)

type Status string

const StatusScheduled Status = "scheduled"

type ServiceProposal interface {
	ServiceProposalID() int
	ServiceProposalAmount() int64
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
