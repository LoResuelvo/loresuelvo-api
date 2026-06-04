package conversation

import "context"

// MessagePublisher is implemented by the realtime adapter to broadcast message events.
type MessagePublisher interface {
	PublishMessage(ctx context.Context, conversation Conversation, senderAuthID string, message Message)
}
