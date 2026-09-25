package operation

import (
	"fmt"
	"math"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

const (
	DefaultInboxLimit = 20
	MaxInboxLimit     = 100
)

type InboxPosition struct {
	StartedOn time.Time
	ID        readmodel.ID
}

type InboxQuery struct {
	Filter InboxFilter
	Limit  int
	After  *InboxPosition
}

type InboxPage struct {
	Operations []readmodel.OperationSummary
	Next       *InboxPosition
}

func (query InboxQuery) Validate() error {
	if err := query.Filter.Validate(); err != nil {
		return err
	}
	if query.Limit < 0 || query.Limit > MaxInboxLimit {
		return fmt.Errorf("%w: limit must be at most %d, or zero for the default", ErrInvalidInboxQuery, MaxInboxLimit)
	}
	if after := query.After; after != nil {
		validKind := after.ID.Kind == readmodel.KindJobRequest || after.ID.Kind == readmodel.KindServiceProposal
		if !validKind || after.ID.ResourceID <= 0 || after.ID.ResourceID > math.MaxInt32 || after.StartedOn.IsZero() {
			return fmt.Errorf("%w: position is invalid", ErrInvalidInboxQuery)
		}
	}
	return nil
}

func (query InboxQuery) EffectiveLimit() int {
	if query.Limit == 0 {
		return DefaultInboxLimit
	}
	return query.Limit
}
