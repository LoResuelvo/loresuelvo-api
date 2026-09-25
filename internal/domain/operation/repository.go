package operation

import (
	"context"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// TimeWindow is half-open: [From, To).
type TimeWindow struct {
	From time.Time
	To   time.Time
}

type InboxCriteria struct {
	Now                  time.Time
	PendingRequestCutoff time.Time
	StalledCutoff        time.Time
	Filter               InboxFilter
	ScheduledWindow      *TimeWindow
	After                *InboxPosition
	Limit                int
}

// InboxReader returns operations ordered by started_on, kind and resource ID, all descending.
type InboxReader interface {
	FindPage(ctx context.Context, criteria InboxCriteria) ([]readmodel.OperationSummary, error)
}
