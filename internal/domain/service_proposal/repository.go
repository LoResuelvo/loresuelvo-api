package serviceproposal

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type ConversationRepository interface {
	FindBetween(providerID int, consumerID int) (conversation.Conversation, error)
}

type ServiceProposalRepository interface {
}

type ProviderRepository interface {
	FindByAuthID(auth0ID string) (*provider.Provider, error)
}

type ConsumerRepository interface {
	FindByID(consumerID int) (*consumer.Consumer, error)
}
