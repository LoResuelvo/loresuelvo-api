package conversation

import "context"

// MessagePageReader reads a bounded page without participant read-state changes.
type MessagePageReader interface {
	FindPage(ctx context.Context, conversationID, limit int) ([]Message, error)
}
