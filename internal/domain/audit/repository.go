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
