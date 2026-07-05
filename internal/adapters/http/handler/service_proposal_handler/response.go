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

type serviceProposalSummaryResponse struct {
	ID             int                                `json:"id"`
	ConversationID int                                `json:"conversation_id"`
	AmountCents    int64                              `json:"amount_cents"`
	ScheduledOn    time.Time                          `json:"scheduled_on"`
	Description    string                             `json:"description"`
	Status         string                             `json:"status"`
	CreatedOn      time.Time                          `json:"created_on"`
	Counterpart    serviceProposalCounterpartResponse `json:"counterpart"`
}

type serviceProposalCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name,omitempty"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}
