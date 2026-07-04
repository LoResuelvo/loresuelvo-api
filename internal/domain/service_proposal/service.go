package serviceproposal

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Service struct {
	repository             ServiceProposalRepository
	providerRepository     ProviderRepository
	consumerRepository     ConsumerRepository
	conversationRepository ConversationRepository
	clock                  clock.Clock
}

func NewService(serviceRepo ServiceProposalRepository, providerRepo ProviderRepository, consumerRepo ConsumerRepository, conversationRepo ConversationRepository, clock clock.Clock) *Service {
	return &Service{
		repository:             serviceRepo,
		providerRepository:     providerRepo,
		consumerRepository:     consumerRepo,
		conversationRepository: conversationRepo,
		clock:                  clock,
	}
}

func (s *Service) CreateServiceProposal(auth0ID string, consumerID int, amount int64, scheduledOn time.Time, description string) (*ServiceProposal, error) {
	provider, consumer, conversation, err := s.getParticipants(auth0ID, consumerID)
	if err != nil {
		return nil, err
	}

	serviceProposal, err := NewServiceProposal(provider, consumer, conversation, amount, scheduledOn, description, s.clock)
	if err != nil {
		return nil, err
	}

	return s.repository.Save(serviceProposal)
}

func (s *Service) getParticipants(providerAuth0ID string, consumerID int) (*provider.Provider, *consumer.Consumer, conversation.Conversation, error) {
	provider, err := s.providerRepository.FindByAuthID(providerAuth0ID)
	if err != nil {
		return nil, nil, nil, ErrProviderRequired
	}

	consumer, err := s.consumerRepository.FindByID(consumerID)
	if err != nil {
		return nil, nil, nil, ErrConsumerRequired
	}

	conversation, err := s.conversationRepository.FindBetween(consumer.ID, provider.ID)
	if err != nil {
		return nil, nil, nil, ErrConversationRequired
	}

	return provider, consumer, conversation, nil
}
