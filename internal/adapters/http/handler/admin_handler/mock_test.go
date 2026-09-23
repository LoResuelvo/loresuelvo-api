package admin_handler

import (
	"context"

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

func (service *serviceMock) ListProviders(ctx context.Context, query string) ([]readmodel.Provider, error) {
	args := service.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]readmodel.Provider), args.Error(1)
}
