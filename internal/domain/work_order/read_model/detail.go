package readmodel

import "time"

type WorkOrderDetail struct {
	ID                int
	ServiceProposalID int
	ConsumerID        int
	ProviderID        int
	Amount            int64
	ScheduledOn       time.Time
	Description       string
	Status            string
	AcceptedOn        time.Time
	PaidOn            time.Time
	CompletionReport  *CompletionReport
}
