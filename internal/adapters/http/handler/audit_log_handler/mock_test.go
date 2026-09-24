package audit_log_handler

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct{ mock.Mock }

func (service *serviceMock) Query(ctx context.Context, authSubject, correlationID string, filter audit.LogFilter) ([]*audit.Event, error) {
	args := service.Called(ctx, authSubject, correlationID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*audit.Event), args.Error(1)
}
