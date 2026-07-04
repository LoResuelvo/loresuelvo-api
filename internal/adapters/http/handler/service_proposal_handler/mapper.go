package service_proposal_handler

import serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"

func serviceProposalCreationResponseFromDomain(proposal *serviceproposal.ServiceProposal) serviceProposalCreationResponse {
	response := serviceProposalCreationResponse{
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
