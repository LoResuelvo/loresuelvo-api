package audit

import (
	"fmt"
	"math"
	"time"
)

// LogFilter selects audit events. Nil fields leave a dimension unrestricted.
// The time window is [OccurredFrom, OccurredTo).
type LogFilter struct {
	OperatorID   *int
	Action       *Action
	ResourceType *string
	ResourceID   *string
	Result       *Result
	OccurredFrom *time.Time
	OccurredTo   *time.Time
}

func (filter LogFilter) Validate() error {
	if filter.OperatorID != nil && (*filter.OperatorID <= 0 || *filter.OperatorID > math.MaxInt32) {
		return fmt.Errorf("%w: operator ID is out of range", ErrInvalidQuery)
	}
	if filter.Action != nil && !validAction(*filter.Action) {
		return fmt.Errorf("%w: action is invalid", ErrInvalidQuery)
	}
	if filter.ResourceType != nil && !validSnakeCase(*filter.ResourceType, 64) {
		return fmt.Errorf("%w: resource type must be bounded snake_case", ErrInvalidQuery)
	}
	if filter.ResourceID != nil && !validSafeIdentifier(*filter.ResourceID, 128) {
		return fmt.Errorf("%w: resource ID is invalid", ErrInvalidQuery)
	}
	if filter.Result != nil && !validResult(*filter.Result) {
		return fmt.Errorf("%w: result is invalid", ErrInvalidQuery)
	}
	if filter.OccurredFrom != nil && filter.OccurredFrom.IsZero() {
		return fmt.Errorf("%w: occurrence start is invalid", ErrInvalidQuery)
	}
	if filter.OccurredTo != nil && filter.OccurredTo.IsZero() {
		return fmt.Errorf("%w: occurrence end is invalid", ErrInvalidQuery)
	}
	if filter.OccurredFrom != nil && filter.OccurredTo != nil && !filter.OccurredFrom.Before(*filter.OccurredTo) {
		return fmt.Errorf("%w: occurrence start must precede end", ErrInvalidQuery)
	}
	return nil
}

func validAction(action Action) bool {
	switch action {
	case ActionCreate, ActionAccess, ActionExecute:
		return true
	default:
		return false
	}
}

func validResult(result Result) bool {
	switch result {
	case ResultSucceeded, ResultPrepared, ResultFailed:
		return true
	default:
		return false
	}
}
