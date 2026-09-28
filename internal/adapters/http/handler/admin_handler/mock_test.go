package admin_handler

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct{ mock.Mock }

func (service *serviceMock) ListConsumers(ctx context.Context, query string) ([]readmodel.Consumer, error) {
	args := service.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]readmodel.Consumer), args.Error(1)
}

func (service *serviceMock) ListProviders(ctx context.Context, filter admin.ProviderDirectoryFilter) ([]readmodel.Provider, error) {
	args := service.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]readmodel.Provider), args.Error(1)
}

type diagnosticServiceMock struct{ mock.Mock }

func (m *diagnosticServiceMock) Query(ctx context.Context, id, subject, correlation string) (*readmodel.ProviderDiagnostic, error) {
	args := m.Called(ctx, id, subject, correlation)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*readmodel.ProviderDiagnostic), args.Error(1)
}

type consumerHistoryServiceMock struct{ mock.Mock }

func (m *consumerHistoryServiceMock) Query(ctx context.Context, id int, q admin.ConsumerHistoryQuery, subject, correlation string) (*readmodel.ConsumerHistory, error) {
	a := m.Called(ctx, id, q, subject, correlation)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*readmodel.ConsumerHistory), a.Error(1)
}
