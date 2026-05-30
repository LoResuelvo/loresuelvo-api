package conversation

import "context"

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
