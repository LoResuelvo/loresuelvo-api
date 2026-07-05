package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateServiceProposal(t *testing.T) {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	serviceRepo := new(MockServiceProposalRepository)
	clock := new(MockClock)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{BaseUser: &user.BaseUser{ID: validProviderID}}, nil)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(&consumer.Consumer{BaseUser: &user.BaseUser{ID: validConsumerID}}, nil)

	conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(validConversation, nil)

	serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(&serviceproposal.ServiceProposal{}, nil)

	clock.
		On("Now").
		Return(time.Now())

	service := serviceproposal.NewService(serviceRepo, &MockUserRepository{provider: providerRepo, consumer: consumerRepo}, conversationRepo, clock)

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
	serviceRepo := new(MockServiceProposalRepository)
	clock := new(MockClock)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(nil, serviceproposal.ErrProviderRequired)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(nil, serviceproposal.ErrConsumerRequired)

	conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(nil, serviceproposal.ErrConversationRequired)

	serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(&serviceproposal.ServiceProposal{}, nil)

	service := serviceproposal.NewService(serviceRepo, &MockUserRepository{provider: providerRepo, consumer: consumerRepo}, conversationRepo, clock)

	serviceProposal, err := service.CreateServiceProposal(
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}

func TestCreateProposalShouldPersist(t *testing.T) {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	serviceRepo := new(MockServiceProposalRepository)
	clock := new(MockClock)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{BaseUser: &user.BaseUser{ID: validProviderID}}, nil)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(&consumer.Consumer{BaseUser: &user.BaseUser{ID: validConsumerID}}, nil)

	conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(validConversation, nil)

	serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(&serviceproposal.ServiceProposal{}, nil).
		Once()

	clock.
		On("Now").
		Return(time.Now())

	service := serviceproposal.NewService(serviceRepo, &MockUserRepository{provider: providerRepo, consumer: consumerRepo}, conversationRepo, clock)

	_, _ = service.CreateServiceProposal(
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	serviceRepo.AssertExpectations(t)
}
