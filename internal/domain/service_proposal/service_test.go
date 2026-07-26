package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type serviceProposalTestEnv struct {
	providerRepo       *MockProviderRepository
	consumerRepo       *MockConsumerRepository
	conversationRepo   *MockConversationRepository
	serviceRepo        *MockServiceProposalRepository
	notificationRepo   *MockNotificationRepository
	notificator        *MockNotificator
	clock              *MockClock
	userRepo           *MockUserRepository
	fileURLResolver    *MockFileURLResolver
	paymentAccountRepo *MockPaymentAccountRepository
}

func setupServiceProposalTest(t *testing.T) *serviceProposalTestEnv {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)
	conversationRepo := new(MockConversationRepository)
	serviceRepo := new(MockServiceProposalRepository)
	notificationRepo := new(MockNotificationRepository)
	notificator := new(MockNotificator)
	clock := new(MockClock)
	fileURLResolver := new(MockFileURLResolver)
	fileURLResolver.
		On("ResolvePublicURLs", mock.Anything, mock.Anything).
		Return(map[string]string{}, nil)
	paymentAccountRepo := new(MockPaymentAccountRepository)
	paymentAccountRepo.
		On("FindByProviderID", mock.Anything, validProviderID, paymentaccount.PaymentProvider("mercado_pago")).
		Return(validPaymentAccount(t), nil)

	userRepo := &MockUserRepository{
		provider: providerRepo,
		consumer: consumerRepo,
	}
	userRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{
			BaseUser: user.RehydrateBaseUser(validProviderID, "", "", "", "", "", nil),
		}, nil)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{
			BaseUser: user.RehydrateBaseUser(validProviderID, "", "", "", "", "", nil),
		}, nil)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(&consumer.Consumer{
			BaseUser: user.RehydrateBaseUser(validConsumerID, "", "", "", "", "", nil),
		}, nil)

	conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(validConversation, nil)

	savedProposal := validSavedServiceProposal()
	serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(savedProposal, nil)

	serviceRepo.
		On("FindByUserID", mock.Anything, validProviderID).
		Return([]*serviceproposal.ServiceProposal{}, nil)

	notificationRepo.
		On("Save", mock.Anything, mock.AnythingOfType("*notification.Notification")).
		Return(&notification.Notification{ID: 1}, nil)

	notificator.
		On("Notify", mock.Anything, mock.AnythingOfType("*notification.Notification")).
		Return(nil)

	clock.
		On("Now").
		Return(time.Now())

	return &serviceProposalTestEnv{
		providerRepo:       providerRepo,
		consumerRepo:       consumerRepo,
		conversationRepo:   conversationRepo,
		serviceRepo:        serviceRepo,
		notificationRepo:   notificationRepo,
		notificator:        notificator,
		clock:              clock,
		userRepo:           userRepo,
		fileURLResolver:    fileURLResolver,
		paymentAccountRepo: paymentAccountRepo,
	}
}

func validPaymentAccount(t *testing.T) *paymentaccount.PaymentAccount {
	t.Helper()
	account, err := paymentaccount.NewPaymentAccount(
		validProviderID,
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-provider",
		[]byte("encrypted-access-token"),
		nil,
		time.Now().Add(time.Hour),
	)
	require.NoError(t, err)
	return account
}

func validSavedServiceProposal() *serviceproposal.ServiceProposal {
	return &serviceproposal.ServiceProposal{
		ID:           77,
		Provider:     &provider.Provider{BaseUser: user.RehydrateBaseUser(validProviderID, "", "", "", "", "", nil)},
		Consumer:     &consumer.Consumer{BaseUser: user.RehydrateBaseUser(validConsumerID, "", "", "", "", "", nil)},
		Conversation: validConversation,
		BookingTerms: validBookingTerms(),
		ScheduledOn:  validServiceScheduledOn,
		Description:  validServiceDescription,
		Status:       serviceproposal.StatusPending,
	}
}

func validBookingTerms() serviceproposal.BookingTerms {
	terms, err := serviceproposal.NewBookingPolicy().Calculate(validServiceAmount, validServiceScheduledOn)
	if err != nil {
		panic(err)
	}
	return terms
}

func (env *serviceProposalTestEnv) newService() *serviceproposal.Service {
	return serviceproposal.NewService(
		env.serviceRepo,
		env.userRepo,
		env.conversationRepo,
		env.notificationRepo,
		env.notificator,
		env.fileURLResolver,
		env.paymentAccountRepo,
		paymentaccount.PaymentProvider("mercado_pago"),
		serviceproposal.NewBookingPolicy(),
		env.clock,
	)
}

func TestCreateServiceProposalRequiresConnectedPaymentAccount(t *testing.T) {
	env := setupServiceProposalTest(t)
	resetMocks(&env.paymentAccountRepo.Mock, &env.serviceRepo.Mock)
	env.paymentAccountRepo.
		On("FindByProviderID", mock.Anything, validProviderID, paymentaccount.PaymentProvider("mercado_pago")).
		Return(nil, paymentaccount.ErrConnectionNotFound).
		Once()

	proposal, err := env.newService().CreateServiceProposal(
		t.Context(),
		validProviderAuth0ID,
		validConsumerID,
		validServiceAmount,
		validServiceScheduledOn,
		validServiceDescription,
	)

	require.ErrorIs(t, err, serviceproposal.ErrPaymentAccountConnectionRequired)
	assert.Nil(t, proposal)
	env.serviceRepo.AssertNotCalled(t, "Save", mock.Anything)
	env.paymentAccountRepo.AssertExpectations(t)
}

func TestCreateServiceProposalReturnsPaymentAccountLookupError(t *testing.T) {
	env := setupServiceProposalTest(t)
	resetMocks(&env.paymentAccountRepo.Mock, &env.serviceRepo.Mock)
	env.paymentAccountRepo.
		On("FindByProviderID", mock.Anything, validProviderID, paymentaccount.PaymentProvider("mercado_pago")).
		Return(nil, assert.AnError).
		Once()

	proposal, err := env.newService().CreateServiceProposal(
		t.Context(), validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription,
	)

	require.ErrorIs(t, err, assert.AnError)
	assert.ErrorContains(t, err, "finding provider payment account")
	assert.Nil(t, proposal)
	env.serviceRepo.AssertNotCalled(t, "Save", mock.Anything)
	env.paymentAccountRepo.AssertExpectations(t)
}

func TestCreateServiceProposal(t *testing.T) {
	env := setupServiceProposalTest(t)
	resetMocks(&env.serviceRepo.Mock)
	env.serviceRepo.
		On("Save", mock.MatchedBy(func(proposal *serviceproposal.ServiceProposal) bool {
			terms := proposal.BookingTerms
			return terms.Currency() == "ARS" &&
				terms.ServiceTotalCents() == validServiceAmount &&
				terms.DepositCents() == 200 &&
				terms.RemainingServiceBalanceCents() == 800 &&
				terms.PlatformFeeTotalCents() == 500000 &&
				terms.PlatformFeeDueNowCents() == 100000 &&
				terms.RemainingPlatformFeeCents() == 400000 &&
				terms.AmountDueNowCents() == 100200 &&
				terms.RemainingAmountDueCents() == 400800 &&
				terms.ContractTotalCents() == 501000
		})).
		Return(validSavedServiceProposal(), nil).
		Once()
	service := env.newService()

	serviceProposal, err := service.CreateServiceProposal(
		t.Context(),
		validProviderAuth0ID,
		validConsumerID,
		validServiceAmount,
		validServiceScheduledOn,
		validServiceDescription,
	)

	require.NoError(t, err)
	require.NotNil(t, serviceProposal)
	env.serviceRepo.AssertExpectations(t)
}

func TestCreateProposalShouldPersist(t *testing.T) {
	env := setupServiceProposalTest(t)
	resetMocks(&env.serviceRepo.Mock)
	env.serviceRepo.
		On("Save", mock.AnythingOfType("*serviceproposal.ServiceProposal")).
		Return(validSavedServiceProposal(), nil).
		Once()

	service := env.newService()

	_, err := service.CreateServiceProposal(
		t.Context(),
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
	env := setupServiceProposalTest(t)
	resetMocks(&env.conversationRepo.Mock)
	env.conversationRepo.
		On("FindBetween", validConsumerID, validProviderID).
		Return(nil, serviceproposal.ErrConversationRequired).
		Once()

	service := env.newService()

	serviceProposal, err := service.CreateServiceProposal(
		t.Context(),
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
	env := setupServiceProposalTest(t)
	resetMocks(&env.notificationRepo.Mock)
	env.notificationRepo.
		On("Save", mock.Anything, mock.MatchedBy(func(notif *notification.Notification) bool {
			return notif != nil &&
				notif.UserID == validConsumerID &&
				notif.Type == notification.TypeServiceProposalReceived &&
				notif.ResourceType == notification.ResourceServiceProposal &&
				notif.ResourceID == validSavedServiceProposal().ID
		})).
		Return(&notification.Notification{ID: 1}, nil).
		Once()

	service := env.newService()

	_, _ = service.CreateServiceProposal(
		t.Context(),
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	env.notificationRepo.AssertExpectations(t)
}

func TestCreationOfProposalShouldNotifyAfterSavingNotification(t *testing.T) {
	env := setupServiceProposalTest(t)
	savedNotification := &notification.Notification{ID: 1, UserID: validConsumerID, ResourceID: validSavedServiceProposal().ID}
	resetMocks(&env.notificationRepo.Mock, &env.notificator.Mock)
	env.notificationRepo.
		On("Save", mock.Anything, mock.AnythingOfType("*notification.Notification")).
		Return(savedNotification, nil).
		Once()
	env.notificator.
		On("Notify", mock.Anything, savedNotification).
		Return(nil).
		Once()

	_, err := env.newService().CreateServiceProposal(
		t.Context(),
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	require.NoError(t, err)
	env.notificationRepo.AssertExpectations(t)
	env.notificator.AssertExpectations(t)
}

func TestCreationOfProposalShouldNotNotifyWhenNotificationCannotBeSaved(t *testing.T) {
	env := setupServiceProposalTest(t)
	resetMocks(&env.notificationRepo.Mock, &env.notificator.Mock)
	env.notificationRepo.
		On("Save", mock.Anything, mock.AnythingOfType("*notification.Notification")).
		Return(nil, assert.AnError).
		Once()

	createdProposal, err := env.newService().CreateServiceProposal(
		t.Context(),
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, createdProposal)
	env.notificationRepo.AssertExpectations(t)
	env.notificator.AssertNotCalled(t, "Notify", mock.Anything, mock.Anything)
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
		Return(&provider.Provider{BaseUser: user.RehydrateBaseUser(validProviderID, "", "", "", "", "", nil)}, nil)

	serviceRepo.
		On("FindByUserID", mock.Anything, validProviderID).
		Return([]*serviceproposal.ServiceProposal{}, nil)

	fileURLResolver := new(MockFileURLResolver)
	fileURLResolver.On("ResolvePublicURLs", mock.Anything, mock.Anything).Return(map[string]string{}, nil)
	paymentAccountRepo := new(MockPaymentAccountRepository)
	service := serviceproposal.NewService(
		serviceRepo, userRepo, conversationRepo, notificationRepo,
		nil, fileURLResolver, paymentAccountRepo, paymentaccount.PaymentProvider("mercado_pago"),
		serviceproposal.NewBookingPolicy(), clock,
	)

	serviceProposals, err := service.GetServiceProposals(t.Context(), validProviderAuth0ID)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposals)
	assert.Empty(t, serviceProposals)
}

func TestConsumerGetsPendingServiceProposal(t *testing.T) {
	env := setupServiceProposalTest(t)
	expectedProposal := &serviceproposal.ServiceProposal{
		ID:           1,
		Provider:     &provider.Provider{BaseUser: user.RehydrateBaseUser(validProviderID, "", "", "Juan", "Gomez", provider.Role, &filedomain.Image{FileID: "provider-photo"})},
		Consumer:     &consumer.Consumer{BaseUser: user.RehydrateBaseUser(validConsumerID, "", "", "", "", consumer.Role, nil)},
		BookingTerms: validBookingTerms(),
		ScheduledOn:  validServiceScheduledOn,
		Description:  validServiceDescription,
		Status:       serviceproposal.StatusPending,
		CreatedOn:    time.Now(),
		Conversation: &conversation.WorkConversation{
			BaseConversation: conversation.RehydrateBaseConversation(10, conversation.TypeWork, "", time.Time{}, nil),
		},
	}
	expectedProposal.Provider.Category = &category.Category{Name: "Plomeria"}

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
	assert.Same(t, expectedProposal, proposals[0])
	assert.Equal(t, expectedProposal.ID, proposals[0].ID)
	assert.Equal(t, "Juan", proposals[0].Provider.Name())
	assert.Equal(t, "Gomez", proposals[0].Provider.Surname())
	assert.Equal(t, "Plomeria", proposals[0].Provider.Categoryname())
	assert.Equal(t, "https://cdn/provider.jpg", proposals[0].Provider.ProfilePhoto().URL)
	env.userRepo.AssertExpectations(t)
	env.serviceRepo.AssertExpectations(t)
}
