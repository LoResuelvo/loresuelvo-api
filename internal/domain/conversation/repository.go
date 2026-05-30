package conversation

import (
	"context"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
)

type Repository interface {
	ExistsBetween(consumerID, providerID int) (bool, error)
	SaveWithMessage(conversation Conversation, message Message) (*Conversation, error)
	FindByID(ctx context.Context, conversationID int) (*Conversation, error)
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

type SummaryReader interface {
	FindByConsumerID(ctx context.Context, consumerID int) ([]readmodel.ConversationSummary, error)
	FindByProviderID(ctx context.Context, providerID int) ([]readmodel.ConversationSummary, error)
}
