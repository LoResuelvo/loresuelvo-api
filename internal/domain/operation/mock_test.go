package operation_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type inboxReaderMock struct{ mock.Mock }

func (m *inboxReaderMock) FindPage(ctx context.Context, criteria operation.InboxCriteria) ([]readmodel.OperationSummary, error) {
	args := m.Called(ctx, criteria)
	if operations := args.Get(0); operations != nil {
		return operations.([]readmodel.OperationSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

type inboxFixedClock struct{ now time.Time }

func (clock inboxFixedClock) Now() time.Time { return clock.now }
