package conversation

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Repository interface {
	ExistsBetween(consumerID, providerID int) (bool, error)
	SaveWithMessage(conversation Conversation, message Message) (*Conversation, error)
	FindByID(ctx context.Context, conversationID int) (*Conversation, error)
	FindByConsumerID(ctx context.Context, consumerID int) ([]Conversation, error)
	FindByProviderID(ctx context.Context, providerID int) ([]Conversation, error)
}

type ConsumerIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
}

type ProviderExistenceChecker interface {
	ExistsByID(id int) (bool, error)
}

type ProviderIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
}

type ProviderFinder interface {
	FindByIDs(ctx context.Context, ids []int) ([]provider.Provider, error)
}

type ConsumerFinder interface {
	FindByIDs(ctx context.Context, ids []int) ([]consumer.Consumer, error)
}

type MessageFinder interface {
	FindLastMessagesByConversationIDs(ctx context.Context, conversationIDs []int) (map[int]Message, error)
}
