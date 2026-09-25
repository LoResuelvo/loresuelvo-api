package operation

import (
	"fmt"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

const (
	DefaultInboxLimit = 20
	MaxInboxLimit     = 100
)

// InboxPosition is the last visible ordering key on a page.
type InboxPosition struct {
	StartedOn time.Time
	ID        readmodel.ID
}

// InboxQuery describes a bounded, newest-first read of the operations inbox.
type InboxQuery struct {
	Limit int
	After *InboxPosition
}

// InboxPage contains domain read models, not an HTTP representation.
type InboxPage struct {
	Operations []readmodel.OperationSummary
	Next       *InboxPosition
}

func (query InboxQuery) Validate() error {
	if query.Limit < 0 || query.Limit > MaxInboxLimit {
		return fmt.Errorf("%w: limit must be at most %d, or zero for the default", ErrInvalidInboxQuery, MaxInboxLimit)
	}
	return nil
}

func (query InboxQuery) effectiveLimit() int {
	if query.Limit == 0 {
		return DefaultInboxLimit
	}
	return query.Limit
}
