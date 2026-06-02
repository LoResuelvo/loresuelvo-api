package jobrequest

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"

type Repository interface {
	SaveWithConversation(jobRequest JobRequest, pendingConversation conversation.Conversation) (*JobRequest, error)
	FindByUserID(userAuthID string) ([]JobRequest, error)
}

type ConsumerRepository interface {
	FindIDByAuthID(authID string) (int, error)
}

type ProviderRepository interface {
	ExistsByID(id int) (bool, error)
}

type ConversationRepository interface {
	ExistsBetween(consumerID, providerID int) (bool, error)
}
