package payment

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/stretchr/testify/mock"
)

type adminPaymentReaderMock struct{ mock.Mock }

func (m *adminPaymentReaderMock) FindPage(ctx context.Context, q AdminPaymentQuery) (*rm.AdminPaymentSnapshot, error) {
	args := m.Called(ctx, q)
	var result *rm.AdminPaymentSnapshot
	if args.Get(0) != nil {
		result = args.Get(0).(*rm.AdminPaymentSnapshot)
	}
	return result, args.Error(1)
}

type adminPaymentOperatorIDFinderMock struct{ mock.Mock }

func (m *adminPaymentOperatorIDFinderMock) FindOperatorIDByAuthID(ctx context.Context, id string) (int, error) {
	a := m.Called(ctx, id)
	return a.Int(0), a.Error(1)
}

type adminPaymentAuditWriterMock struct{ mock.Mock }

func (m *adminPaymentAuditWriterMock) Save(ctx context.Context, e *audit.Event) error {
	return m.Called(ctx, e).Error(0)
}

type adminPaymentClockMock struct{ mock.Mock }

func (m *adminPaymentClockMock) Now() time.Time { return m.Called().Get(0).(time.Time) }
