package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/assert"
)

func TestCreateServiceProposal(t *testing.T) {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	clock := new(MockClock)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{ID: validProviderID}, nil)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(&consumer.Consumer{ID: validConsumerID}, nil)

	conversationRepo.
		On("FindBetween", validProviderID, validConsumerID).
		Return(validConversation, nil)

	clock.
		On("Now").
		Return(time.Now())

	service := serviceproposal.NewService(providerRepo, consumerRepo, conversationRepo, clock)

	serviceProposal, err := service.CreateServiceProposal(
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposal)
}

func TestCreateServiceProposalWithNoConversation(t *testing.T) {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	clock := new(MockClock)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(nil, serviceproposal.ErrProviderRequired)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(nil, serviceproposal.ErrConsumerRequired)

	conversationRepo.
		On("FindBetween", validProviderID, validConsumerID).
		Return(nil, serviceproposal.ErrConversationRequired)

	service := serviceproposal.NewService(providerRepo, consumerRepo, conversationRepo, clock)

	serviceProposal, err := service.CreateServiceProposal(
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}
