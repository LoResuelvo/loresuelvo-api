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

type operationResponse struct {
	ID              string                   `json:"id"`
	Stage           readmodel.Stage          `json:"stage"`
	StartedOn       time.Time                `json:"started_on"`
	JobRequest      *jobRequestResponse      `json:"job_request"`
	ServiceProposal *serviceProposalResponse `json:"service_proposal"`
	WorkOrder       *workOrderResponse       `json:"work_order"`
}

type jobRequestResponse struct {
	ID     int               `json:"id"`
	Status jobrequest.Status `json:"status"`
}

type serviceProposalResponse struct {
	ID     int                    `json:"id"`
	Status serviceproposal.Status `json:"status"`
}

type workOrderResponse struct {
	ID     int              `json:"id"`
	Status workorder.Status `json:"status"`
}

// operationID renders the stable identifier of the resource that started the
// operation, for example "jr-12" or "sp-34".
func operationID(id readmodel.ID) string {
	return string(id.Kind) + "-" + strconv.Itoa(id.ResourceID)
}

func operationResponsesFromDomain(operations []readmodel.OperationSummary) []operationResponse {
	responses := make([]operationResponse, 0, len(operations))
	for _, found := range operations {
		response := operationResponse{
			ID:        operationID(found.ID),
			Stage:     found.Stage,
			StartedOn: found.StartedOn.UTC(),
		}
		if found.JobRequest != nil {
			response.JobRequest = &jobRequestResponse{ID: found.JobRequest.ID, Status: found.JobRequest.Status}
		}
		if found.ServiceProposal != nil {
			response.ServiceProposal = &serviceProposalResponse{ID: found.ServiceProposal.ID, Status: found.ServiceProposal.Status}
		}
		if found.WorkOrder != nil {
			response.WorkOrder = &workOrderResponse{ID: found.WorkOrder.ID, Status: found.WorkOrder.Status}
		}
		responses = append(responses, response)
	}
	return responses
}
