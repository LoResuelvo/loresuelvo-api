package conversation

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Service struct {
	conversationRepository   Repository
	consumerIDFinder        ConsumerIDFinder
	providerFinder          ProviderFinder
	providerExistenceChecker ProviderExistenceChecker
	providerIDFinder        ProviderIDFinder
	consumerFinder          ConsumerFinder
	messageFinder           MessageFinder
}

func NewService(
	conversationRepository Repository,
	consumerIDFinder ConsumerIDFinder,
	providerFinder ProviderFinder,
	providerExistenceChecker ProviderExistenceChecker,
	providerIDFinder ProviderIDFinder,
	consumerFinder ConsumerFinder,
	messageFinder MessageFinder,
) *Service {
	return &Service{
		conversationRepository:   conversationRepository,
		consumerIDFinder:         consumerIDFinder,
		providerFinder:           providerFinder,
		providerExistenceChecker: providerExistenceChecker,
		providerIDFinder:         providerIDFinder,
		consumerFinder:           consumerFinder,
		messageFinder:            messageFinder,
	}
}

func (s *Service) StartWorkRequest(consumerAuthID string, providerID int, content string) (*Conversation, error) {
	consumerID, err := s.consumerIDForWorkRequest(consumerAuthID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureProviderExists(providerID); err != nil {
		return nil, err
	}

	conversation, message, err := newPendingWorkRequest(consumerID, providerID, content)
	if err != nil {
		return nil, err
	}

	if err := s.ensureConversationDoesNotExist(consumerID, providerID); err != nil {
		return nil, err
	}

	return s.conversationRepository.SaveWithMessage(*conversation, *message)
}

func (s *Service) GetByID(ctx context.Context, authID string, conversationID int) (*Conversation, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if !s.authenticatedUserCanAccessConversation(authID, *foundConversation) {
		return nil, ErrConversationAccessDenied
	}

	return foundConversation, nil
}

func (s *Service) List(ctx context.Context, authID string) ([]ConversationSummary, error) {
	if consumerID, err := s.consumerIDFinder.FindIDByAuthID(authID); err == nil {
		return s.listForConsumer(ctx, consumerID)
	}

	if providerID, err := s.providerIDFinder.FindIDByAuthID(authID); err == nil {
		return s.listForProvider(ctx, providerID)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) listForConsumer(ctx context.Context, consumerID int) ([]ConversationSummary, error) {
	conversations, err := s.conversationRepository.FindByConsumerID(ctx, consumerID)
	if err != nil {
		return nil, err
	}
	if len(conversations) == 0 {
		return []ConversationSummary{}, nil
	}

	providerIDs := make([]int, len(conversations))
	for i, conv := range conversations {
		providerIDs[i] = conv.ProviderID
	}

	providers, err := s.providerFinder.FindByIDs(ctx, providerIDs)
	if err != nil {
		return nil, err
	}
	providerMap := make(map[int]provider.Provider)
	for _, p := range providers {
		providerMap[p.ID] = p
	}

	lastMessages, err := s.messageFinder.FindLastMessagesByConversationIDs(ctx, conversationIDsFrom(conversations))
	if err != nil {
		return nil, err
	}

	return BuildConsumerSummaries(conversations, providerMap, lastMessages), nil
}

func (s *Service) listForProvider(ctx context.Context, providerID int) ([]ConversationSummary, error) {
	conversations, err := s.conversationRepository.FindByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if len(conversations) == 0 {
		return []ConversationSummary{}, nil
	}

	consumerIDs := make([]int, len(conversations))
	for i, conv := range conversations {
		consumerIDs[i] = conv.ConsumerID
	}

	consumers, err := s.consumerFinder.FindByIDs(ctx, consumerIDs)
	if err != nil {
		return nil, err
	}
	consumerMap := make(map[int]consumer.Consumer)
	for _, c := range consumers {
		consumerMap[c.ID] = c
	}

	lastMessages, err := s.messageFinder.FindLastMessagesByConversationIDs(ctx, conversationIDsFrom(conversations))
	if err != nil {
		return nil, err
	}

	return BuildProviderSummaries(conversations, consumerMap, lastMessages), nil
}

func conversationIDsFrom(conversations []Conversation) []int {
	ids := make([]int, len(conversations))
	for i, conv := range conversations {
		ids[i] = conv.ID
	}
	return ids
}

func (s *Service) consumerIDForWorkRequest(consumerAuthID string) (int, error) {
	consumerID, err := s.consumerIDFinder.FindIDByAuthID(consumerAuthID)
	if err != nil {
		return 0, ErrOnlyConsumerCanStartWorkRequest
	}

	return consumerID, nil
}

func (s *Service) ensureProviderExists(providerID int) error {
	if providerID <= 0 {
		return ErrProviderRequired
	}

	providerExists, err := s.providerExistenceChecker.ExistsByID(providerID)
	if err != nil {
		return err
	}
	if !providerExists {
		return ErrProviderDoesNotExist
	}

	return nil
}

func (s *Service) ensureConversationDoesNotExist(consumerID, providerID int) error {
	exists, err := s.conversationRepository.ExistsBetween(consumerID, providerID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyExists
	}

	return nil
}

func (s *Service) authenticatedUserCanAccessConversation(authID string, conversation Conversation) bool {
	return s.authenticatedConsumerMatches(authID, conversation.ConsumerID) ||
		s.authenticatedProviderMatches(authID, conversation.ProviderID)
}

func (s *Service) authenticatedConsumerMatches(authID string, consumerID int) bool {
	authenticatedConsumerID, err := s.consumerIDFinder.FindIDByAuthID(authID)
	return err == nil && authenticatedConsumerID == consumerID
}

func (s *Service) authenticatedProviderMatches(authID string, providerID int) bool {
	authenticatedProviderID, err := s.providerIDFinder.FindIDByAuthID(authID)
	return err == nil && authenticatedProviderID == providerID
}

func newPendingWorkRequest(consumerID, providerID int, content string) (*Conversation, *Message, error) {
	conversation, err := NewPendingConversation(consumerID, providerID)
	if err != nil {
		return nil, nil, err
	}

	message, err := NewConsumerMessage(content)
	if err != nil {
		return nil, nil, err
	}

	return conversation, message, nil
}