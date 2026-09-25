package operation

import (
	"context"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// InboxCriteria is a validated read of the inbox. Alerts are evaluated at Now
// using the cutoffs the domain derived from it.
type InboxCriteria struct {
	Now                  time.Time
	PendingRequestCutoff time.Time
	StalledCutoff        time.Time
	Filter               InboxFilter
	After                *InboxPosition
	Limit                int
}

// InboxReader retrieves operations ordered by start time descending, then by
// kind and resource ID descending. It returns at most criteria.Limit
// operations after criteria.After that match criteria.Filter, with alerts
// evaluated at criteria.Now and without multiplying them by one-to-many
// relations.
type InboxReader interface {
	FindPage(ctx context.Context, criteria InboxCriteria) ([]readmodel.OperationSummary, error)
}
