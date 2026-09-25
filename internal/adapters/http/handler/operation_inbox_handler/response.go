package operation_inbox_handler

import (
	"strconv"
	"time"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type pageResponse struct {
	Operations []operationResponse `json:"operations"`
	NextCursor *string             `json:"next_cursor"`
}

// operationResponse is an allowlisted summary: it never carries messages,
// attachments, credentials, biometrics or payment payloads.
type operationResponse struct {
	ID                    string                   `json:"id"`
	Stage                 readmodel.Stage          `json:"stage"`
	StartedOn             time.Time                `json:"started_on"`
	JobRequest            *jobRequestResponse      `json:"job_request"`
	ServiceProposal       *serviceProposalResponse `json:"service_proposal"`
	WorkOrder             *workOrderResponse       `json:"work_order"`
	Consumer              partyResponse            `json:"consumer"`
	Provider              partyResponse            `json:"provider"`
	Category              *categoryResponse        `json:"category"`
	Alerts                []readmodel.Alert        `json:"alerts"`
	NextActionOwner       *readmodel.Owner         `json:"next_action_owner"`
	LastBusinessAdvanceOn *time.Time               `json:"last_business_advance_on"`
	Limitations           []readmodel.Limitation   `json:"limitations"`
}

type jobRequestResponse struct {
	ID        int               `json:"id"`
	Status    jobrequest.Status `json:"status"`
	CreatedOn time.Time         `json:"created_on"`
}

type serviceProposalResponse struct {
	ID                       int                    `json:"id"`
	Status                   serviceproposal.Status `json:"status"`
	CreatedOn                time.Time              `json:"created_on"`
	ScheduledOn              time.Time              `json:"scheduled_on"`
	EstimatedDurationMinutes int                    `json:"estimated_duration_minutes"`
	BookingPaymentDeadline   time.Time              `json:"booking_payment_deadline"`
}

type workOrderResponse struct {
	ID                   int              `json:"id"`
	Status               workorder.Status `json:"status"`
	AcceptedOn           time.Time        `json:"accepted_on"`
	CompletionReportedOn *time.Time       `json:"completion_reported_on"`
	BalancePaidOn        *time.Time       `json:"balance_paid_on"`
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

// operationID renders the stable identifier of the resource that started the
// operation, for example "jr-12" or "sp-34".
func operationID(id readmodel.ID) string {
	return string(id.Kind) + "-" + strconv.Itoa(id.ResourceID)
}

func optionalUTC(instant *time.Time) *time.Time {
	if instant == nil {
		return nil
	}
	utc := instant.UTC()
	return &utc
}

func operationResponsesFromDomain(operations []readmodel.OperationSummary) []operationResponse {
	responses := make([]operationResponse, 0, len(operations))
	for _, found := range operations {
		response := operationResponse{
			ID:                    operationID(found.ID),
			Stage:                 found.Stage,
			StartedOn:             found.StartedOn.UTC(),
			Consumer:              partyResponse{ID: found.Consumer.ID, Name: found.Consumer.Name, Surname: found.Consumer.Surname},
			Provider:              partyResponse{ID: found.Provider.ID, Name: found.Provider.Name, Surname: found.Provider.Surname},
			Alerts:                append([]readmodel.Alert{}, found.Alerts...),
			NextActionOwner:       found.NextActionOwner,
			LastBusinessAdvanceOn: optionalUTC(found.LastBusinessAdvanceOn),
			Limitations:           append([]readmodel.Limitation{}, found.Limitations...),
		}
		if request := found.JobRequest; request != nil {
			response.JobRequest = &jobRequestResponse{ID: request.ID, Status: request.Status, CreatedOn: request.CreatedOn.UTC()}
		}
		if proposal := found.ServiceProposal; proposal != nil {
			response.ServiceProposal = &serviceProposalResponse{
				ID: proposal.ID, Status: proposal.Status, CreatedOn: proposal.CreatedOn.UTC(),
				ScheduledOn: proposal.ScheduledOn.UTC(), EstimatedDurationMinutes: proposal.EstimatedDurationMinutes,
				BookingPaymentDeadline: proposal.BookingPaymentDeadline.UTC(),
			}
		}
		if order := found.WorkOrder; order != nil {
			response.WorkOrder = &workOrderResponse{
				ID: order.ID, Status: order.Status, AcceptedOn: order.AcceptedOn.UTC(),
				CompletionReportedOn: optionalUTC(order.CompletionReportedOn), BalancePaidOn: optionalUTC(order.BalancePaidOn),
			}
		}
		if category := found.Category; category != nil {
			response.Category = &categoryResponse{ID: category.ID, Name: category.Name}
		}
		responses = append(responses, response)
	}
	return responses
}
