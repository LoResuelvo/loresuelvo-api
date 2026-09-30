package httpadapter

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/mock"
)

type collectionServiceMock struct{ mock.Mock }

func (m *collectionServiceMock) Summary(ctx context.Context, authID string, input provider.ActivityQueryInput) (*readmodel.Collections, error) {
	args := m.Called(ctx, authID, input)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.Collections), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *collectionServiceMock) Detail(ctx context.Context, authID string, input provider.CollectionDetailInput) (*readmodel.CollectionDetail, int, error) {
	args := m.Called(ctx, authID, input)
	if value := args.Get(0); value != nil {
		return value.(*readmodel.CollectionDetail), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}
