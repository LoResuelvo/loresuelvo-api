package service_proposal_handler

import "time"

type serviceProposalCreationResponse struct {
	ID             int                  `json:"id"`
	ConversationID int                  `json:"conversation_id"`
	ConsumerID     int                  `json:"consumer_id"`
	ProviderID     int                  `json:"provider_id"`
	AmountCents    int64                `json:"amount_cents"`
	ScheduledOn    time.Time            `json:"scheduled_on"`
	Description    string               `json:"description"`
	Status         string               `json:"status"`
	BookingTerms   bookingTermsResponse `json:"booking_terms"`
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
	BookingTerms   bookingTermsResponse               `json:"booking_terms"`
}

type bookingTermsResponse struct {
	Currency                     string    `json:"currency"`
	ServiceTotalCents            int64     `json:"service_total_cents"`
	DepositCents                 int64     `json:"deposit_cents"`
	RemainingServiceBalanceCents int64     `json:"remaining_service_balance_cents"`
	PlatformFeeTotalCents        int64     `json:"platform_fee_total_cents"`
	PlatformFeeDueNowCents       int64     `json:"platform_fee_due_now_cents"`
	RemainingPlatformFeeCents    int64     `json:"remaining_platform_fee_cents"`
	AmountDueNowCents            int64     `json:"amount_due_now_cents"`
	RemainingAmountDueCents      int64     `json:"remaining_amount_due_cents"`
	ContractTotalCents           int64     `json:"contract_total_cents"`
	BookingPaymentDeadline       time.Time `json:"booking_payment_deadline"`
}

type serviceProposalCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name,omitempty"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}
