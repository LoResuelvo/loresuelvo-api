package service_proposal_handler

import "time"

type serviceProposalCreationResponse struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	ConsumerID     int       `json:"consumer_id"`
	ProviderID     int       `json:"provider_id"`
	AmountCents    int64     `json:"amount_cents"`
	ScheduledOn    time.Time `json:"scheduled_on"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
}
