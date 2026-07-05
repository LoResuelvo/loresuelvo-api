package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type serviceProposalTestEnv struct {
	providerRepo     *MockProviderRepository
	consumerRepo     *MockConsumerRepository
	conversationRepo *MockConversationRepository
	serviceRepo      *MockServiceProposalRepository
	notificationRepo *MockNotificationRepository
	clock            *MockClock
	userRepo         *MockUserRepository
	fileURLResolver  *MockFileURLResolver
}

func setupServiceProposalTest() *serviceProposalTestEnv {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	serviceRepo := new(MockServiceProposalRepository)
	notificationRepo := new(MockNotificationRepository)
	clock := new(MockClock)
	fileURLResolver := new(MockFileURLResolver)
	fileURLResolver.
		On("ResolvePublicURLs", mock.Anything, mock.Anything).
		Return(map[string]string{}, nil)

	userRepo := &MockUserRepository{
		provider: providerRepo,
		consumer: consumerRepo,
	}
	userRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{
			BaseUser: &user.BaseUser{ID: validProviderID},
		}, nil)

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

	serviceRepo.
		On("FindByUserID", mock.Anything, validProviderID).
		Return([]*serviceproposal.ServiceProposal{}, nil)

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
		fileURLResolver:  fileURLResolver,
	}
}

func (env *serviceProposalTestEnv) newService() *serviceproposal.Service {
	return serviceproposal.NewService(
		env.serviceRepo,
		env.userRepo,
		env.conversationRepo,
		env.notificationRepo,
		env.fileURLResolver,
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

func TestGetServiceProposalsWithoutServiceProposals(t *testing.T) {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	serviceRepo := new(MockServiceProposalRepository)
	notificationRepo := new(MockNotificationRepository)
	clock := new(MockClock)

	userRepo := &MockUserRepository{provider: providerRepo, consumer: consumerRepo}
	userRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{BaseUser: &user.BaseUser{ID: validProviderID}}, nil)

	serviceRepo.
		On("FindByUserID", mock.Anything, validProviderID).
		Return([]*serviceproposal.ServiceProposal{}, nil)

	fileURLResolver := new(MockFileURLResolver)
	fileURLResolver.On("ResolvePublicURLs", mock.Anything, mock.Anything).Return(map[string]string{}, nil)
	service := serviceproposal.NewService(serviceRepo, userRepo, conversationRepo, notificationRepo, fileURLResolver, clock)

	serviceProposals, err := service.GetServiceProposals(t.Context(), validProviderAuth0ID)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposals)
	assert.Empty(t, serviceProposals)
}

func TestConsumerGetsPendingServiceProposal(t *testing.T) {
	env := setupServiceProposalTest()
	expectedProposal := &serviceproposal.ServiceProposal{
		ID:          1,
		Provider:    &provider.Provider{BaseUser: &user.BaseUser{ID: validProviderID}},
		Consumer:    &consumer.Consumer{BaseUser: &user.BaseUser{ID: validConsumerID, Role: consumer.Role}},
		Amount:      validServiceAmount,
		ScheduledOn: validServiceScheduledOn,
		Description: validServiceDescription,
		Status:      serviceproposal.StatusPending,
		CreatedOn:   time.Now(),
		Conversation: &conversation.WorkConversation{
			BaseConversation: &conversation.BaseConversation{ID: 10},
		},
	}
	expectedProposal.Provider.BaseUser.Name = "Juan"
	expectedProposal.Provider.BaseUser.Surname = "Gomez"
	expectedProposal.Provider.Category = &category.Category{Name: "Plomeria"}
	expectedProposal.Provider.ProfilePhotoFileID = "provider-photo"

	resetMocks(&env.userRepo.Mock, &env.serviceRepo.Mock)
	env.userRepo.
		On("FindByAuthID", validConsumerAuth0ID).
		Return(expectedProposal.Consumer, nil).
		Once()
	env.serviceRepo.
		On("FindByUserID", mock.Anything, validConsumerID).
		Return([]*serviceproposal.ServiceProposal{expectedProposal}, nil).
		Once()
	resetMocks(&env.fileURLResolver.Mock)
	env.fileURLResolver.
		On("ResolvePublicURLs", mock.Anything, []string{"provider-photo"}).
		Return(map[string]string{"provider-photo": "https://cdn/provider.jpg"}, nil).
		Once()

	proposals, err := env.newService().GetServiceProposals(t.Context(), validConsumerAuth0ID)

	require.NoError(t, err)
	require.Len(t, proposals, 1)
	assert.Equal(t, expectedProposal.ID, proposals[0].ID)
	assert.Equal(t, "Juan", proposals[0].Counterpart.Name)
	assert.Equal(t, "Gomez", proposals[0].Counterpart.Surname)
	assert.Equal(t, "Plomeria", proposals[0].Counterpart.CategoryName)
	assert.Equal(t, "https://cdn/provider.jpg", proposals[0].Counterpart.ProfilePhotoURL)
	env.userRepo.AssertExpectations(t)
	env.serviceRepo.AssertExpectations(t)
}
