package operation_chat_handler

import (
	"context"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type chatServiceMock struct{ mock.Mock }

func (m *chatServiceMock) Query(ctx context.Context, id, subject, correlation, reason string) (*readmodel.OperationChat, error) {
	args := m.Called(ctx, id, subject, correlation, reason)
	var result *readmodel.OperationChat
	if args.Get(0) != nil {
		result = args.Get(0).(*readmodel.OperationChat)
	}
	return result, args.Error(1)
}
