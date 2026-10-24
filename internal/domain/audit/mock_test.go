package audit_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/mock"
)

type logReaderMock struct{ mock.Mock }

func (m *logReaderMock) CaptureWatermark(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *logReaderMock) FindPage(ctx context.Context, filter audit.LogFilter, watermark int64, before *audit.LogPosition, limit int) ([]*audit.Event, error) {
	args := m.Called(ctx, filter, watermark, before, limit)
	if events := args.Get(0); events != nil {
		return events.([]*audit.Event), args.Error(1)
	}
	return nil, args.Error(1)
}

type operatorNameFinderMock struct{ mock.Mock }

func (m *operatorNameFinderMock) FindNamesByIDs(ctx context.Context, ids []int) (map[int]string, error) {
	args := m.Called(ctx, ids)
	if names := args.Get(0); names != nil {
		return names.(map[int]string), args.Error(1)
	}
	return nil, args.Error(1)
}

type auditWriterMock struct{ mock.Mock }

func (m *auditWriterMock) Save(ctx context.Context, event *audit.Event) error {
	return m.Called(ctx, event).Error(0)
}

type auditOperatorIDFinderMock struct{ mock.Mock }

func (m *auditOperatorIDFinderMock) FindOperatorIDByAuthID(ctx context.Context, authID string) (int, error) {
	args := m.Called(ctx, authID)
	return args.Int(0), args.Error(1)
}

type auditFixedClock struct{ now time.Time }

func (clock auditFixedClock) Now() time.Time { return clock.now }
