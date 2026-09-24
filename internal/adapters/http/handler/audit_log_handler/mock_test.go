package audit_log_handler

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct{ mock.Mock }

func (service *serviceMock) Query(ctx context.Context, authSubject, correlationID string, query audit.LogQuery) (audit.LogPage, error) {
	args := service.Called(ctx, authSubject, correlationID, query)
	return args.Get(0).(audit.LogPage), args.Error(1)
}
