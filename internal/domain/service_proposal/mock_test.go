package serviceproposal_test

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/mock"
)

var (
	validConsumerID               = 1
	validProviderAuth0ID          = "provider-auth0-id"
	validServiceDescription       = "Service description"
	validServiceAmount      int64 = 1000
	validServiceScheduledOn       = time.Now()
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

func (m *MockConsumerRepository) FindByID(id int) (*consumer.Consumer, error) {
	args := m.Called(id)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*consumer.Consumer), args.Error(1)
}
