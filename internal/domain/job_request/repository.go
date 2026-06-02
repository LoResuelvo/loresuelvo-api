package jobrequest

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"

type Repository interface {
	SaveWithPendingConversation(jobRequest JobRequest, pendingConversation conversation.Conversation) (*JobRequest, error)
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
