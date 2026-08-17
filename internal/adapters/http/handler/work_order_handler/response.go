package work_order_handler

import "time"

type workOrderSummaryResponse struct {
	ID                int                          `json:"id"`
	ServiceProposalID int                          `json:"service_proposal_id"`
	AmountCents       int64                        `json:"amount_cents"`
	ScheduledOn       time.Time                    `json:"scheduled_on"`
	Description       string                       `json:"description"`
	Status            string                       `json:"status"`
	AcceptedOn        time.Time                    `json:"accepted_on"`
	Counterpart       workOrderCounterpartResponse `json:"counterpart"`
}

type workOrderDetailResponse struct {
	ID                int                       `json:"id"`
	ServiceProposalID int                       `json:"service_proposal_id"`
	ConsumerID        int                       `json:"consumer_id"`
	ProviderID        int                       `json:"provider_id"`
	AmountCents       int64                     `json:"amount_cents"`
	ScheduledOn       time.Time                 `json:"scheduled_on"`
	Description       string                    `json:"description"`
	Status            string                    `json:"status"`
	AcceptedOn        time.Time                 `json:"accepted_on"`
	PaidOn            *time.Time                `json:"paid_on,omitempty"`
	CompletionReport  *completionReportResponse `json:"completion_report,omitempty"`
	Review            *reviewResponse           `json:"review,omitempty"`
}

type workOrderCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name,omitempty"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}

type completionReportResponse struct {
	ID          int                       `json:"id"`
	Description string                    `json:"description"`
	ReportedOn  time.Time                 `json:"reported_on"`
	Images      []completionImageResponse `json:"images"`
}

type completionImageResponse struct {
	FileID       string `json:"file_id"`
	OriginalName string `json:"original_name"`
	URL          string `json:"url"`
}

type reviewResponse struct {
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}
