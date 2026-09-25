package operation_inbox_handler

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct{ mock.Mock }

func (service *serviceMock) Query(ctx context.Context, query operation.InboxQuery) (operation.InboxPage, error) {
	args := service.Called(ctx, query)
	return args.Get(0).(operation.InboxPage), args.Error(1)
}
