package readmodel

import "time"

type WorkOrderSummary struct {
	ID                int
	ServiceProposalID int
	Amount            int64
	ScheduledOn       time.Time
	Description       string
	Status            string
	AcceptedOn        time.Time
	Counterpart       Counterpart
}

type Counterpart struct {
	ID                 int
	Role               string
	Name               string
	Surname            string
	CategoryName       string
	ProfilePhotoFileID string
	ProfilePhotoURL    string
}
