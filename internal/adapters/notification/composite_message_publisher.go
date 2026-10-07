package notificationadapter

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

type CompositeMessagePublisher struct {
	channels []conversation.MessagePublisher
}

func NewCompositeMessagePublisher(channels ...conversation.MessagePublisher) *CompositeMessagePublisher {
	return &CompositeMessagePublisher{channels: channels}
}
func (p *CompositeMessagePublisher) PublishMessage(ctx context.Context, conv conversation.Conversation, senderAuthID string, message conversation.Message) {
	for _, channel := range p.channels {
		if channel != nil {
			channel.PublishMessage(ctx, conv, senderAuthID, message)
		}
	}
}
