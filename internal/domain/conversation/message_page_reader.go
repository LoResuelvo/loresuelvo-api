package conversation

import (
	"context"
	"time"
)

// MessagePosition is the final delivered chronological ordering key.
type MessagePosition struct {
	CreatedOn time.Time
	ID        int
}

// MessagePageReader reads a bounded page without participant read-state changes.
// The limit includes the caller's optional lookahead row.
type MessagePageReader interface {
	FindPage(ctx context.Context, conversationID int, after *MessagePosition, limit int) ([]Message, error)
}
