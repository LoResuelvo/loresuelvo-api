package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type serviceProposalTestEnv struct {
	providerRepo     *MockProviderRepository
	consumerRepo     *MockConsumerRepository
	conversationRepo *MockConversationRepository
	serviceRepo      *MockServiceProposalRepository
	notificationRepo *MockNotificationRepository
	clock            *MockClock
	userRepo         *MockUserRepository
}

func setupServiceProposalTest() *serviceProposalTestEnv {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	serviceRepo := new(MockServiceProposalRepository)
	notificationRepo := new(MockNotificationRepository)
	clock := new(MockClock)

	userRepo := &MockUserRepository{
		provider: providerRepo,
		consumer: consumerRepo,
	}

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{
			BaseUser: &user.BaseUser{ID: validProviderID},
		}, nil)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(&consumer.Consumer{
			BaseUser: &user.BaseUser{ID: validConsumerID},
		}, nil)

	conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(validConversation, nil)

	serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(&serviceproposal.ServiceProposal{}, nil)

	notificationRepo.
		On("Save", mock.AnythingOfType("*notification.Notification")).
		Return(&notification.Notification{ID: 1}, nil)

	clock.
		On("Now").
		Return(time.Now())

	return &serviceProposalTestEnv{
		providerRepo:     providerRepo,
		consumerRepo:     consumerRepo,
		conversationRepo: conversationRepo,
		serviceRepo:      serviceRepo,
		notificationRepo: notificationRepo,
		clock:            clock,
		userRepo:         userRepo,
	}
}

func (env *serviceProposalTestEnv) newService() *serviceproposal.Service {
	return serviceproposal.NewService(
		env.serviceRepo,
		env.userRepo,
		env.conversationRepo,
		env.notificationRepo,
		env.clock,
	)
}

func TestCreateServiceProposal(t *testing.T) {
	env := setupServiceProposalTest()
	service := env.newService()

	serviceProposal, err := service.CreateServiceProposal(
		validProviderAuth0ID,
		validConsumerID,
		validServiceAmount,
		validServiceScheduledOn,
		validServiceDescription,
	)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposal)
}

func TestCreateProposalShouldPersist(t *testing.T) {
	env := setupServiceProposalTest()
	resetMocks(&env.serviceRepo.Mock)
	env.serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(&serviceproposal.ServiceProposal{}, nil).
		Once()

	service := env.newService()

	_, err := service.CreateServiceProposal(
		validProviderAuth0ID,
		validConsumerID,
		validServiceAmount,
		validServiceScheduledOn,
		validServiceDescription,
	)

	assert.NoError(t, err)
	env.serviceRepo.AssertExpectations(t)
}

func TestCreateServiceProposalWithNoConversation(t *testing.T) {
	env := setupServiceProposalTest()
	resetMocks(&env.conversationRepo.Mock)
	env.conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(nil, serviceproposal.ErrConversationRequired).
		Once()

	service := env.newService()

	serviceProposal, err := service.CreateServiceProposal(
		validProviderAuth0ID,
		validConsumerID,
		validServiceAmount,
		validServiceScheduledOn,
		validServiceDescription,
	)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
	assert.ErrorIs(t, err, serviceproposal.ErrConversationRequired)

	env.conversationRepo.AssertExpectations(t)
}
func TestCreationOfProposalShouldCreateNotification(t *testing.T) {
	env := setupServiceProposalTest()
	resetMocks(&env.notificationRepo.Mock)
	env.notificationRepo.
		On("Save", mock.AnythingOfType("*notification.Notification")).
		Return(&notification.Notification{ID: 1}, nil).
		Once()

	service := env.newService()

	_, _ = service.CreateServiceProposal(
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	env.notificationRepo.AssertExpectations(t)
}
