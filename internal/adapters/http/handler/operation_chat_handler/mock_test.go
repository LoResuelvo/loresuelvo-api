package operation_chat_handler

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type chatServiceMock struct{ mock.Mock }

func (m *chatServiceMock) Query(ctx context.Context, id, subject, correlation, reason string, query operation.ChatQuery) (*readmodel.OperationChat, error) {
	args := m.Called(ctx, id, subject, correlation, reason, query)
	var result *readmodel.OperationChat
	if args.Get(0) != nil {
		result = args.Get(0).(*readmodel.OperationChat)
	}
	return result, args.Error(1)
}
