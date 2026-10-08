package work_order_handler

import (
	"context"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/mock"
)

type reviewReporterMock struct{ mock.Mock }

func (m *reviewReporterMock) Report(ctx context.Context, authID string, id int, category, explanation string) (*workorder.ReviewReport, error) {
	a := m.Called(ctx, authID, id, category, explanation)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*workorder.ReviewReport), a.Error(1)
}
