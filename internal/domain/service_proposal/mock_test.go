package serviceproposal_test

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/mock"
)

var (
	validConsumerID               = 1
	validProviderID               = 1
	validProviderAuth0ID          = "provider-auth0-id"
	validServiceDescription       = "Service description"
	validServiceAmount      int64 = 1000
	validServiceScheduledOn       = time.Now().Add(time.Hour)
	validConversation             = &conversation.WorkConversation{BaseConversation: &conversation.BaseConversation{Status: conversation.StatusActive}}
)

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
	provider *MockProviderRepository
	consumer *MockConsumerRepository
}

func (m *MockUserRepository) FindProviderByAuthID(auth0ID string) (*provider.Provider, error) {
	return m.provider.FindByAuthID(auth0ID)
}

func (m *MockUserRepository) FindConsumerByID(id int) (*consumer.Consumer, error) {
	return m.consumer.FindByID(id)
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
