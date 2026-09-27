package operation_detail_handler

import (
	"strconv"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type detailResponse struct {
	ID                string                     `json:"id"`
	StartedOn         time.Time                  `json:"started_on"`
	JobRequest        *jobRequestResponse        `json:"job_request"`
	ServiceProposal   *proposalResponse          `json:"service_proposal"`
	RelatedProposals  []relatedProposalResponse  `json:"related_proposals"`
	WorkOrder         *workOrderResponse         `json:"work_order"`
	PaymentMilestones []paymentMilestoneResponse `json:"payment_milestones"`
	Timeline          []timelineResponse         `json:"timeline"`
	Consumer          partyResponse              `json:"consumer"`
	Provider          partyResponse              `json:"provider"`
	Category          *categoryResponse          `json:"category"`
	Address           *addressResponse           `json:"address"`
	SourceAssessment  *assessmentResponse        `json:"source_assessment"`
}
type jobRequestResponse struct {
	ID          int                    `json:"id"`
	Status      string                 `json:"status"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	CreatedOn   time.Time              `json:"created_on"`
	Images      []privateImageResponse `json:"images"`
}
type privateImageResponse struct {
	FileID       string    `json:"file_id"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	Purpose      string    `json:"purpose"`
	CreatedOn    time.Time `json:"created_on"`
}
type proposalResponse struct {
	ID                       int       `json:"id"`
	Status                   string    `json:"status"`
	Description              string    `json:"description"`
	AmountCents              int64     `json:"amount_cents"`
	Currency                 string    `json:"currency"`
	CreatedOn                time.Time `json:"created_on"`
	ScheduledOn              time.Time `json:"scheduled_on"`
	EstimatedDurationMinutes int       `json:"estimated_duration_minutes"`
	DepositCents             int64     `json:"deposit_cents"`
	PlatformFeeTotalCents    int64     `json:"platform_fee_total_cents"`
	PlatformFeeDueNowCents   int64     `json:"platform_fee_due_now_cents"`
	ServiceBalanceCents      int64     `json:"service_balance_cents"`
	PlatformFeeBalanceCents  int64     `json:"platform_fee_balance_cents"`
}
type relatedProposalResponse struct {
	OperationID string `json:"operation_id"`
	proposalResponse
}
type workOrderResponse struct {
	ID                   int                       `json:"id"`
	Status               string                    `json:"status"`
	AcceptedOn           time.Time                 `json:"accepted_on"`
	CompletionReportedOn *time.Time                `json:"completion_reported_on"`
	BalancePaidOn        *time.Time                `json:"balance_paid_on"`
	CompletionReport     *completionReportResponse `json:"completion_report"`
	Review               *workOrderReviewResponse  `json:"review"`
}
type completionReportResponse struct {
	Description string                 `json:"description"`
	ReportedOn  time.Time              `json:"reported_on"`
	Images      []privateImageResponse `json:"images"`
}
type workOrderReviewResponse struct {
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}
type paymentMilestoneResponse struct {
	ID        string    `json:"id"`
	Purpose   string    `json:"purpose"`
	Status    string    `json:"status"`
	CreatedOn time.Time `json:"created_on"`
}
type timelineResponse struct {
	Type       string    `json:"type"`
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	OccurredOn time.Time `json:"occurred_on"`
}
type partyResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}
type categoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type addressResponse struct {
	Street       string  `json:"street"`
	StreetNumber string  `json:"street_number"`
	Floor        *string `json:"floor"`
	Unit         *string `json:"unit"`
	Source       string  `json:"source"`
}
type assessmentResponse struct {
	ID               int               `json:"id"`
	Version          int               `json:"version"`
	Outcome          string            `json:"outcome"`
	Category         *categoryResponse `json:"category"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	BasedOnMessageID int               `json:"based_on_message_id"`
	CreatedOn        time.Time         `json:"created_on"`
}

func responseFromDomain(found *readmodel.OperationDetail) detailResponse {
	response := detailResponse{
		ID: string(found.ID.Kind) + "-" + strconv.Itoa(found.ID.ResourceID), StartedOn: found.StartedOn.UTC(),
		Consumer: partyResponse(found.Consumer), Provider: partyResponse(found.Provider),
		RelatedProposals:  make([]relatedProposalResponse, 0, len(found.RelatedProposals)),
		PaymentMilestones: make([]paymentMilestoneResponse, 0, len(found.PaymentMilestones)),
		Timeline:          make([]timelineResponse, 0, len(found.Timeline)),
	}
	if found.Category != nil {
		response.Category = &categoryResponse{ID: found.Category.ID, Name: found.Category.Name}
	}
	if request := found.JobRequest; request != nil {
		response.JobRequest = &jobRequestResponse{ID: request.ID, Status: request.Status, Title: request.Title, Description: request.Description, CreatedOn: request.CreatedOn.UTC(), Images: privateImagesFromDomain(request.Images)}
	}
	if proposal := found.ServiceProposal; proposal != nil {
		mapped := proposalFromDomain(*proposal)
		response.ServiceProposal = &mapped
	}
	for _, related := range found.RelatedProposals {
		response.RelatedProposals = append(response.RelatedProposals, relatedProposalResponse{
			OperationID:      string(related.OperationID.Kind) + "-" + strconv.Itoa(related.OperationID.ResourceID),
			proposalResponse: proposalFromDomain(related.Proposal),
		})
	}
	if order := found.WorkOrder; order != nil {
		response.WorkOrder = &workOrderResponse{ID: order.ID, Status: order.Status, AcceptedOn: order.AcceptedOn.UTC(),
			CompletionReportedOn: order.CompletionReportedOn, BalancePaidOn: order.BalancePaidOn}
		if report := order.CompletionReport; report != nil {
			response.WorkOrder.CompletionReport = &completionReportResponse{Description: report.Description, ReportedOn: report.ReportedOn.UTC(), Images: privateImagesFromDomain(report.Images)}
		}
		if review := order.Review; review != nil {
			response.WorkOrder.Review = &workOrderReviewResponse{Rating: review.Rating, Description: review.Description}
		}
	}
	for _, milestone := range found.PaymentMilestones {
		response.PaymentMilestones = append(response.PaymentMilestones, paymentMilestoneResponse(milestone))
	}
	for _, event := range found.Timeline {
		response.Timeline = append(response.Timeline, timelineResponse(event))
	}
	if address := found.Address; address != nil {
		response.Address = &addressResponse{Street: address.Street, StreetNumber: address.StreetNumber, Floor: address.Floor, Unit: address.Unit, Source: "current_consumer_address"}
	}
	if source := found.SourceAssessment; source != nil {
		assessment := &assessmentResponse{ID: source.ID, Version: source.Version, Outcome: source.Outcome, Title: source.Title, Description: source.Description, BasedOnMessageID: source.BasedOnMessageID, CreatedOn: source.CreatedOn.UTC()}
		if source.Category != nil {
			assessment.Category = &categoryResponse{ID: source.Category.ID, Name: source.Category.Name}
		}
		response.SourceAssessment = assessment
	}
	return response
}

func privateImagesFromDomain(images []readmodel.PrivateImage) []privateImageResponse {
	mapped := make([]privateImageResponse, 0, len(images))
	for _, image := range images {
		mapped = append(mapped, privateImageResponse{FileID: image.ID, OriginalName: image.OriginalName,
			MimeType: image.MimeType, Purpose: image.Purpose, CreatedOn: image.CreatedOn.UTC()})
	}
	return mapped
}

func proposalFromDomain(proposal readmodel.DetailProposal) proposalResponse {
	return proposalResponse{ID: proposal.ID, Status: proposal.Status, Description: proposal.Description,
		AmountCents: proposal.AmountCents, Currency: proposal.Currency, CreatedOn: proposal.CreatedOn.UTC(),
		ScheduledOn: proposal.ScheduledOn.UTC(), EstimatedDurationMinutes: proposal.EstimatedDurationMinutes,
		DepositCents: proposal.DepositCents, PlatformFeeTotalCents: proposal.PlatformFeeTotalCents,
		PlatformFeeDueNowCents:  proposal.PlatformFeeDueNowCents,
		ServiceBalanceCents:     proposal.ServiceBalanceCents(),
		PlatformFeeBalanceCents: proposal.PlatformFeeBalanceCents()}
}
