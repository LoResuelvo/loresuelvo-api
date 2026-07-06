package serviceproposal_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/mock"
)

var (
	validConsumerID               = 1
	validProviderID               = 1
	validConsumerAuth0ID          = "consumer-auth0-id"
	validProviderAuth0ID          = "provider-auth0-id"
	validServiceDescription       = "Service description"
	validServiceAmount      int64 = 1000
	validServiceScheduledOn       = time.Now().Add(time.Hour)
	validConversation             = &conversation.WorkConversation{BaseConversation: &conversation.BaseConversation{Status: conversation.StatusActive}}
)

func resetMocks(mocks ...*mock.Mock) {
	for _, m := range mocks {
		m.ExpectedCalls = nil
		m.Calls = nil
	}
}

type MockProviderRepository struct {
	mock.Mock
}

func (m *MockProviderRepository) FindByAuthID(auth0ID string) (*provider.Provider, error) {
	args := m.Called(auth0ID)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*provider.Provider), args.Error(1)
}

type MockConsumerRepository struct {
	mock.Mock
}

type MockUserRepository struct {
	mock.Mock
	provider *MockProviderRepository
	consumer *MockConsumerRepository
}

func (m *MockUserRepository) FindProviderByAuthID(auth0ID string) (*provider.Provider, error) {
	return m.provider.FindByAuthID(auth0ID)
}

func (m *MockUserRepository) FindConsumerByID(id int) (*consumer.Consumer, error) {
	return m.consumer.FindByID(id)
}

func (m *MockUserRepository) FindByAuthID(auth0ID string) (user.User, error) {
	args := m.Called(auth0ID)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(user.User), args.Error(1)
}

func (m *MockConsumerRepository) FindByID(id int) (*consumer.Consumer, error) {
	args := m.Called(id)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*consumer.Consumer), args.Error(1)
}

type MockClock struct {
	mock.Mock
}

type MockFileURLResolver struct {
	mock.Mock
}

func (m *MockFileURLResolver) ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error) {
	args := m.Called(ctx, fileIDs)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockClock) Now() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

type MockConversationRepository struct {
	mock.Mock
}

func (m *MockConversationRepository) FindBetween(consumerID int, providerID int) (conversation.Conversation, error) {
	args := m.Called(consumerID, providerID)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(conversation.Conversation), args.Error(1)
}

type MockServiceProposalRepository struct {
	mock.Mock
}

func (m *MockServiceProposalRepository) Save(serviceProposal *serviceproposal.ServiceProposal) (*serviceproposal.ServiceProposal, error) {
	args := m.Called(serviceProposal)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*serviceproposal.ServiceProposal), args.Error(1)
}

func (m *MockServiceProposalRepository) FindByUserID(ctx context.Context, userID int) ([]*serviceproposal.ServiceProposal, error) {
	args := m.Called(ctx, userID)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]*serviceproposal.ServiceProposal), args.Error(1)
}

func (m *MockServiceProposalRepository) FindByID(ctx context.Context, id int) (*serviceproposal.ServiceProposal, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceproposal.ServiceProposal), args.Error(1)
}

func (m *MockServiceProposalRepository) SaveWithWorkOrder(ctx context.Context, proposal *serviceproposal.ServiceProposal, order *workorder.WorkOrder) (*workorder.WorkOrder, error) {
	args := m.Called(ctx, proposal, order)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workorder.WorkOrder), args.Error(1)
}

type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Save(ctx context.Context, notif *notification.Notification) (*notification.Notification, error) {
	args := m.Called(ctx, notif)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*notification.Notification), args.Error(1)
}

type MockNotificator struct {
	mock.Mock
}

func (m *MockNotificator) Notify(ctx context.Context, notif *notification.Notification) error {
	args := m.Called(ctx, notif)
	return args.Error(0)
}
