package service_proposal_handler

import (
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal/read_model"
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
		response.ConversationID = proposal.Conversation.Base().ID
	}

	return response
}

func serviceProposalSummaryResponsesFromDomain(summaries []readmodel.ServiceProposalSummary) []serviceProposalSummaryResponse {
	responses := make([]serviceProposalSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		responses = append(responses, serviceProposalSummaryResponse{
			ID:             summary.ID,
			ConversationID: summary.ConversationID,
			AmountCents:    summary.Amount,
			ScheduledOn:    summary.ScheduledOn,
			Description:    summary.Description,
			Status:         summary.Status,
			CreatedOn:      summary.CreatedOn,
			Counterpart: serviceProposalCounterpartResponse{
				ID:              summary.Counterpart.ID,
				Role:            summary.Counterpart.Role,
				Name:            summary.Counterpart.Name,
				Surname:         summary.Counterpart.Surname,
				CategoryName:    summary.Counterpart.CategoryName,
				ProfilePhotoURL: summary.Counterpart.ProfilePhotoURL,
			},
		})
	}
	return responses
}
