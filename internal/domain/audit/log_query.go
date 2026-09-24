package audit

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LogPosition is the last visible ordering key on a page.
type LogPosition struct {
	OccurredOn time.Time
	ID         uuid.UUID
}

// LogQuery describes a bounded read. A continued read must carry both its
// original committed-ingest watermark and its last visible position.
type LogQuery struct {
	Filter    LogFilter
	Limit     int
	Watermark *int64
	Before    *LogPosition
}

// LogPage contains domain events, not an HTTP representation.
type LogPage struct {
	Events    []*Event
	Watermark int64
	Next      *LogPosition
}

func (query LogQuery) Validate() error {
	if err := query.Filter.Validate(); err != nil {
		return err
	}
	if query.Limit < 0 || query.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidQuery)
	}
	if (query.Watermark == nil) != (query.Before == nil) {
		return fmt.Errorf("%w: watermark and position must be supplied together", ErrInvalidQuery)
	}
	if query.Watermark != nil && *query.Watermark < 0 {
		return fmt.Errorf("%w: watermark is invalid", ErrInvalidQuery)
	}
	if query.Before != nil && (query.Before.ID == uuid.Nil || query.Before.OccurredOn.IsZero()) {
		return fmt.Errorf("%w: position is invalid", ErrInvalidQuery)
	}
	return nil
}

func (query LogQuery) effectiveLimit() int {
	if query.Limit == 0 {
		return defaultLogQueryLimit
	}
	return query.Limit
}
