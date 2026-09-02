package realtime

import (
	"context"
	"log/slog"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

type userAuthIDFinder interface {
	FindAuthIDByID(id int) (string, error)
}

// Publisher broadcasts message events to connected clients.
type Publisher struct {
	dispatcher     eventDispatcher
	userRepository userAuthIDFinder
}

// NewPublisher creates a new realtime Publisher.
func NewPublisher(dispatcher eventDispatcher, userRepository userAuthIDFinder) *Publisher {
	return &Publisher{
		dispatcher:     dispatcher,
		userRepository: userRepository,
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

	consumerAuthID, err := p.userRepository.FindAuthIDByID(workConversation.ConsumerID)
	if err != nil {
		slog.Warn("realtime publisher: failed to find consumer auth id",
			"consumerID", workConversation.ConsumerID, "error", err)
		return
	}

	providerAuthID, err := p.userRepository.FindAuthIDByID(workConversation.ProviderID)
	if err != nil {
		slog.Warn("realtime publisher: failed to find provider auth id",
			"providerID", workConversation.ProviderID, "error", err)
		return
	}

	event, err := BuildMessageEvent(conv.ID(), message)
	if err != nil {
		slog.Error("realtime publisher: failed to build event", "error", err)
		return
	}

	if consumerAuthID != senderAuthID {
		p.publishToParticipant(ctx, consumerAuthID, conversation.SenderConsumer, workConversation.ConsumerID, event)
	}
	if providerAuthID != senderAuthID {
		p.publishToParticipant(ctx, providerAuthID, conversation.SenderProvider, workConversation.ProviderID, event)
	}
}

func (p *Publisher) publishToParticipant(ctx context.Context, authID, role string, profileID int, payload []byte) {
	if err := p.dispatcher.Publish(ctx, authID, role, profileID, payload); err != nil {
		slog.Warn("realtime publisher: failed to publish distributed event",
			"authID", authID, "role", role, "profileID", profileID, "error", err)
	}
}
