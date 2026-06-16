package conversation

import (
	"context"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
)

type Repository interface {
	SaveConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	FindByID(ctx context.Context, conversationID int) (Conversation, error)
	AddMessage(ctx context.Context, conversationID int, message Message) (*Message, error)
	CountMessagesBySenderRole(ctx context.Context, conversationID int, senderRole string) (int, error)
}

type ConsumerIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
	FindAuthIDByID(id int) (string, error)
}

type ProviderIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
	FindAuthIDByID(id int) (string, error)
}

type Reader interface {
	FindSummariesByConsumerID(ctx context.Context, consumerID int) ([]readmodel.ConversationSummary, error)
	FindSummariesByProviderID(ctx context.Context, providerID int) ([]readmodel.ConversationSummary, error)
	FindDetailByIDForConsumer(ctx context.Context, conversationID int) (*readmodel.ConversationDetail, error)
	FindDetailByIDForProvider(ctx context.Context, conversationID int) (*readmodel.ConversationDetail, error)
}
