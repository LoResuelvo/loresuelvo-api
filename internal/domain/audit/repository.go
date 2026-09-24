package audit

import (
	"context"

	"github.com/google/uuid"
)

// Writer appends an event. It never alters an existing event.
type Writer interface {
	Save(ctx context.Context, event *Event) error
}

// Reader retrieves an event by its immutable identifier.
type Reader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Event, error)
}

// LogReader retrieves a bounded, newest-first view of immutable audit events.
// Nil filter fields leave their dimension unrestricted.
type LogReader interface {
	FindLatest(ctx context.Context, filter LogFilter, limit int) ([]*Event, error)
}

// OperatorIDFinder resolves the local operator from an authenticated subject.
type OperatorIDFinder interface {
	FindOperatorIDByAuthID(ctx context.Context, authID string) (int, error)
}
