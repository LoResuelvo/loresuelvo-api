package admin

import (
	"context"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
)

type ConsumerDirectoryReader interface {
	FindConsumers(ctx context.Context) ([]readmodel.Consumer, error)
}

type ProviderDirectoryReader interface {
	FindProviders(ctx context.Context) ([]readmodel.Provider, error)
}
