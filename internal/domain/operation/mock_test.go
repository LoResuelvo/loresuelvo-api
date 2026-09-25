package operation_test

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type inboxReaderMock struct{ mock.Mock }

func (m *inboxReaderMock) FindPage(ctx context.Context, after *operation.InboxPosition, limit int) ([]readmodel.OperationSummary, error) {
	args := m.Called(ctx, after, limit)
	if operations := args.Get(0); operations != nil {
		return operations.([]readmodel.OperationSummary), args.Error(1)
	}
	return nil, args.Error(1)
}
