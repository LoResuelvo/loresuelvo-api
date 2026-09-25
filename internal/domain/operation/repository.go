package operation

import (
	"context"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// InboxReader retrieves operations ordered by start time descending, then by
// kind and resource ID descending. It returns at most limit operations after
// the given position, without multiplying them by one-to-many relations.
type InboxReader interface {
	FindPage(ctx context.Context, after *InboxPosition, limit int) ([]readmodel.OperationSummary, error)
}
