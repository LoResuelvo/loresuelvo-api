package workorder

import "time"

type Status string

const StatusScheduled Status = "scheduled"

type WorkOrder struct {
	ID                int
	ServiceProposalID int
	ConsumerID        int
	ProviderID        int
	Status            Status
	AcceptedOn        time.Time
}

func New(serviceProposalID, consumerID, providerID int, acceptedOn time.Time) *WorkOrder {
	return &WorkOrder{
		ServiceProposalID: serviceProposalID,
		ConsumerID:        consumerID,
		ProviderID:        providerID,
		Status:            StatusScheduled,
		AcceptedOn:        acceptedOn,
	}
}
