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
	userRepository         UserRepository
	conversationRepository ConversationRepository
	notificationRepository NotificationRepository
	clock                  clock.Clock
}

func NewService(serviceRepo ServiceProposalRepository, userRepo UserRepository, conversationRepo ConversationRepository, notificationRepo NotificationRepository, clock clock.Clock) *Service {
	return &Service{
		repository:             serviceRepo,
		userRepository:         userRepo,
		conversationRepository: conversationRepo,
		notificationRepository: notificationRepo,
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

	notification := serviceProposal.CreateNotification(s.clock)

	_, err = s.notificationRepository.Save(notification)
	if err != nil {
		return nil, err
	}

	return s.repository.Save(serviceProposal)
}

func (s *Service) GetServiceProposals(auth0ID string) ([]*ServiceProposal, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	return s.repository.FindByUserID(foundUser.Base().ID)
}

func (s *Service) getParticipants(providerAuth0ID string, consumerID int) (*provider.Provider, *consumer.Consumer, conversation.Conversation, error) {
	provider, err := s.userRepository.FindProviderByAuthID(providerAuth0ID)
	if err != nil {
		return nil, nil, nil, ErrProviderRequired
	}

	consumer, err := s.userRepository.FindConsumerByID(consumerID)
	if err != nil {
		return nil, nil, nil, ErrConsumerRequired
	}

	conversation, err := s.conversationRepository.FindBetween(consumer.Base().ID, provider.Base().ID)
	if err != nil {
		return nil, nil, nil, ErrConversationRequired
	}

	return provider, consumer, conversation, nil
}
