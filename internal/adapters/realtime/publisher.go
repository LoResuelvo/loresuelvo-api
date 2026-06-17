package realtime

import (
	"context"
	"log/slog"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

// Publisher broadcasts message events to connected clients.
type Publisher struct {
	hub          *Hub
	consumerRepo *repositories.ConsumerRepository
	providerRepo *repositories.ProviderRepository
}

// NewPublisher creates a new realtime Publisher.
func NewPublisher(hub *Hub, consumerRepo *repositories.ConsumerRepository, providerRepo *repositories.ProviderRepository) *Publisher {
	return &Publisher{
		hub:          hub,
		consumerRepo: consumerRepo,
		providerRepo: providerRepo,
	}
}

// PublishMessage broadcasts a message event to the counterpart participant.
func (p *Publisher) PublishMessage(ctx context.Context, conv conversation.Conversation, senderAuthID string, message conversation.Message) {
	workConversation, ok := conv.(*conversation.WorkConversation)
	if !ok {
		slog.Warn("realtime publisher: expected work conversation",
			"conversationType", conv.ConversationType())
		return
	}

	consumerAuthID, err := p.consumerRepo.FindAuthIDByID(workConversation.ConsumerID)
	if err != nil {
		slog.Warn("realtime publisher: failed to find consumer auth id",
			"consumerID", workConversation.ConsumerID, "error", err)
		return
	}

	providerAuthID, err := p.providerRepo.FindAuthIDByID(workConversation.ProviderID)
	if err != nil {
		slog.Warn("realtime publisher: failed to find provider auth id",
			"providerID", workConversation.ProviderID, "error", err)
		return
	}

	event, err := BuildMessageEvent(conv.Base().ID, message.ID, message.SenderRole, message.Content, message.CreatedOn)
	if err != nil {
		slog.Error("realtime publisher: failed to build event", "error", err)
		return
	}

	p.hub.BroadcastMessage(ctx, consumerAuthID, workConversation.ConsumerID, providerAuthID, workConversation.ProviderID, senderAuthID, message.SenderRole, event)
}
