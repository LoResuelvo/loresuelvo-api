package operation_detail_handler

import (
	"strconv"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type detailResponse struct {
	ID               string              `json:"id"`
	StartedOn        time.Time           `json:"started_on"`
	JobRequest       *jobRequestResponse `json:"job_request"`
	ServiceProposal  *proposalResponse   `json:"service_proposal"`
	Consumer         partyResponse       `json:"consumer"`
	Provider         partyResponse       `json:"provider"`
	Category         *categoryResponse   `json:"category"`
	Address          *addressResponse    `json:"address"`
	SourceAssessment *assessmentResponse `json:"source_assessment"`
}
type jobRequestResponse struct {
	ID          int       `json:"id"`
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedOn   time.Time `json:"created_on"`
}
type proposalResponse struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"`
	CreatedOn time.Time `json:"created_on"`
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
	}
	if found.Category != nil {
		response.Category = &categoryResponse{ID: found.Category.ID, Name: found.Category.Name}
	}
	if request := found.JobRequest; request != nil {
		response.JobRequest = &jobRequestResponse{ID: request.ID, Status: request.Status, Title: request.Title, Description: request.Description, CreatedOn: request.CreatedOn.UTC()}
	}
	if proposal := found.ServiceProposal; proposal != nil {
		response.ServiceProposal = &proposalResponse{ID: proposal.ID, Status: proposal.Status, CreatedOn: proposal.CreatedOn.UTC()}
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
