package serviceproposal

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type ConversationRepository interface {
	FindBetween(consumerID int, providerID int) (conversation.Conversation, error)
}

type ServiceProposalRepository interface {
	Save(serviceProposal *ServiceProposal) (*ServiceProposal, error)
}

type UserRepository interface {
	FindProviderByAuthID(auth0ID string) (*provider.Provider, error)
	FindConsumerByID(consumerID int) (*consumer.Consumer, error)
}
