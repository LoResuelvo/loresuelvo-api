package service_proposal_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

func serviceProposalCreationResponseFromDomain(proposal *serviceproposal.ServiceProposal) serviceProposalCreationResponse {
	response := serviceProposalCreationResponse{
		ID:           proposal.ID,
		AmountCents:  proposal.BookingTerms.ServiceTotalCents(),
		ScheduledOn:  proposal.ScheduledOn,
		Description:  proposal.Description,
		Status:       string(proposal.Status),
		BookingTerms: bookingTermsResponseFromDomain(proposal.BookingTerms),
	}

	if proposal.Provider != nil {
		response.ProviderID = proposal.Provider.ID()
	}
	if proposal.Consumer != nil {
		response.ConsumerID = proposal.Consumer.ID()
	}
	if proposal.Conversation != nil {
		response.ConversationID = proposal.Conversation.ID()
	}

	return response
}

func serviceProposalSummaryResponsesFromDomain(proposals []*serviceproposal.ServiceProposal, viewerAuthID string) ([]serviceProposalSummaryResponse, error) {
	responses := make([]serviceProposalSummaryResponse, 0, len(proposals))
	for _, proposal := range proposals {
		counterpart, err := proposal.CounterpartFor(viewerAuthID)
		if err != nil {
			return nil, err
		}
		counterpartResponse := serviceProposalCounterpartResponse{
			ID:      counterpart.ID(),
			Role:    counterpart.Role(),
			Name:    counterpart.Name(),
			Surname: counterpart.Surname(),
		}
		if profilePhoto := counterpart.ProfilePhoto(); profilePhoto != nil {
			counterpartResponse.ProfilePhotoURL = profilePhoto.URL
		}
		if counterpartProvider, ok := counterpart.(*provider.Provider); ok {
			counterpartResponse.CategoryName = counterpartProvider.Categoryname()
		}
		responses = append(responses, serviceProposalSummaryResponse{
			ID:             proposal.ID,
			ConversationID: proposal.Conversation.ID(),
			AmountCents:    proposal.BookingTerms.ServiceTotalCents(),
			ScheduledOn:    proposal.ScheduledOn,
			Description:    proposal.Description,
			Status:         string(proposal.Status),
			CreatedOn:      proposal.CreatedOn,
			Counterpart:    counterpartResponse,
			BookingTerms:   bookingTermsResponseFromDomain(proposal.BookingTerms),
		})
	}
	return responses, nil
}

func bookingTermsResponseFromDomain(terms serviceproposal.BookingTerms) bookingTermsResponse {
	return bookingTermsResponse{
		Currency:                     terms.Currency(),
		ServiceTotalCents:            terms.ServiceTotalCents(),
		DepositCents:                 terms.DepositCents(),
		RemainingServiceBalanceCents: terms.RemainingServiceBalanceCents(),
		PlatformFeeTotalCents:        terms.PlatformFeeTotalCents(),
		PlatformFeeDueNowCents:       terms.PlatformFeeDueNowCents(),
		RemainingPlatformFeeCents:    terms.RemainingPlatformFeeCents(),
		AmountDueNowCents:            terms.AmountDueNowCents(),
		RemainingAmountDueCents:      terms.RemainingAmountDueCents(),
		ContractTotalCents:           terms.ContractTotalCents(),
		BookingPaymentDeadline:       terms.BookingPaymentDeadline(),
	}
}
