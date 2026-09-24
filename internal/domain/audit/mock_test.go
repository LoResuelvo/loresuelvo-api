package audit_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/mock"
)

type logReaderMock struct{ mock.Mock }

func (m *logReaderMock) FindLatest(ctx context.Context, filter audit.LogFilter, limit int) ([]*audit.Event, error) {
	args := m.Called(ctx, filter, limit)
	if events := args.Get(0); events != nil {
		return events.([]*audit.Event), args.Error(1)
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
