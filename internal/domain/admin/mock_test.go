package admin_test

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/stretchr/testify/mock"
)

type consumerDirectoryReaderMock struct{ mock.Mock }

func (reader *consumerDirectoryReaderMock) FindConsumers(ctx context.Context) ([]readmodel.Consumer, error) {
	args := reader.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]readmodel.Consumer), args.Error(1)
}

type profilePhotoURLResolverMock struct{ mock.Mock }

func (resolver *profilePhotoURLResolverMock) ResolvePublicURLs(
	ctx context.Context,
	fileIDs []string,
) (map[string]string, error) {
	args := resolver.Called(ctx, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}
