package readmodel

import "time"

type ServiceProposalSummary struct {
	ID             int
	ConversationID int
	Amount         int64
	ScheduledOn    time.Time
	Description    string
	Status         string
	CreatedOn      time.Time
	Counterpart    Counterpart
}

// TODO: Considerar mover a un helper global para reutilizarlo desde otros módulos
type Counterpart struct {
	ID                 int
	Role               string
	Name               string
	Surname            string
	CategoryName       string
	ProfilePhotoFileID string
	ProfilePhotoURL    string
}
