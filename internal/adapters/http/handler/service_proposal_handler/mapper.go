package service_proposal_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

func serviceProposalCreationResponseFromDomain(proposal *serviceproposal.ServiceProposal) serviceProposalCreationResponse {
	response := serviceProposalCreationResponse{
		ID:          proposal.ID,
		AmountCents: proposal.Amount,
		ScheduledOn: proposal.ScheduledOn,
		Description: proposal.Description,
		Status:      string(proposal.Status),
	}

	if proposal.Provider != nil {
		response.ProviderID = proposal.Provider.ID
	}
	if proposal.Consumer != nil {
		response.ConsumerID = proposal.Consumer.ID
	}
	if proposal.Conversation != nil {
		response.ConversationID = proposal.Conversation.ID()
	}

	return response
}

func workOrderResponseFromDomain(order *workorder.WorkOrder) workOrderResponse {
	return workOrderResponse{
		ID:                order.ID,
		ServiceProposalID: order.ServiceProposalID(),
		ConsumerID:        order.ConsumerID(),
		ProviderID:        order.ProviderID(),
		AmountCents:       order.Amount(),
		ScheduledOn:       order.ScheduledOn(),
		Description:       order.Description(),
		Status:            string(order.Status),
		AcceptedOn:        order.AcceptedOn,
	}
}

func serviceProposalSummaryResponsesFromDomain(proposals []*serviceproposal.ServiceProposal, viewerAuthID string) ([]serviceProposalSummaryResponse, error) {
	responses := make([]serviceProposalSummaryResponse, 0, len(proposals))
	for _, proposal := range proposals {
		counterpart, err := proposal.CounterpartFor(viewerAuthID)
		if err != nil {
			return nil, err
		}
		counterpartBase := counterpart.Base()
		counterpartResponse := serviceProposalCounterpartResponse{
			ID:      counterpartBase.ID,
			Role:    counterpartBase.Role,
			Name:    counterpartBase.Name,
			Surname: counterpartBase.Surname,
		}
		if counterpartBase.ProfilePhoto != nil {
			counterpartResponse.ProfilePhotoURL = counterpartBase.ProfilePhoto.URL
		}
		if counterpartProvider, ok := counterpart.(*provider.Provider); ok {
			counterpartResponse.CategoryName = counterpartProvider.Categoryname()
		}
		responses = append(responses, serviceProposalSummaryResponse{
			ID:             proposal.ID,
			ConversationID: proposal.Conversation.ID(),
			AmountCents:    proposal.Amount,
			ScheduledOn:    proposal.ScheduledOn,
			Description:    proposal.Description,
			Status:         string(proposal.Status),
			CreatedOn:      proposal.CreatedOn,
			Counterpart:    counterpartResponse,
		})
	}
	return responses, nil
}
