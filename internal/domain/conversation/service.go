package conversation

import (
	"context"
	"strings"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
)

type Service struct {
	conversationRepository   Repository
	consumerIDFinder         ConsumerIDFinder
	providerExistenceChecker ProviderExistenceChecker
	providerIDFinder         ProviderIDFinder
	conversationReader       Reader
	messagePublisher         MessagePublisher
	chatbot                  Chatbot
}

func NewService(
	conversationRepository Repository,
	consumerIDFinder ConsumerIDFinder,
	providerExistenceChecker ProviderExistenceChecker,
	providerIDFinder ProviderIDFinder,
	conversationReader Reader,
	messagePublisher MessagePublisher,
	chatbots ...Chatbot,
) *Service {
	var chatbot Chatbot
	if len(chatbots) > 0 {
		chatbot = chatbots[0]
	}

	return &Service{
		conversationRepository:   conversationRepository,
		consumerIDFinder:         consumerIDFinder,
		providerExistenceChecker: providerExistenceChecker,
		providerIDFinder:         providerIDFinder,
		conversationReader:       conversationReader,
		messagePublisher:         messagePublisher,
		chatbot:                  chatbot,
	}
}

// TODO: Eliminar método
func (s *Service) StartWorkRequest(consumerAuthID string, providerID int, content string) (Conversation, error) {
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

	conversation.AddMessage(*message)
	return s.conversationRepository.SaveConversation(context.Background(), conversation)
}

func (s *Service) GetByID(ctx context.Context, authID string, conversationID int) (*readmodel.ConversationDetail, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	workConversation, ok := foundConversation.(*WorkConversation)
	if !ok {
		return nil, ErrConversationAccessDenied
	}

	if s.authenticatedConsumerMatches(authID, workConversation.ConsumerID) {
		return s.conversationReader.FindDetailByIDForConsumer(ctx, conversationID)
	}

	if s.authenticatedProviderMatches(authID, workConversation.ProviderID) {
		return s.conversationReader.FindDetailByIDForProvider(ctx, conversationID)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) SendMessage(ctx context.Context, authID string, conversationID int, content string) (*Message, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	message, err := s.newMessageForAuthenticatedParticipant(authID, foundConversation, content)
	if err != nil {
		return nil, err
	}
	if err := s.ensureMessageAllowedInCurrentConversationState(ctx, foundConversation, *message); err != nil {
		return nil, err
	}
	foundConversation.AddMessage(*message)

	sentMessage, err := s.conversationRepository.AddMessage(ctx, conversationID, *message)
	if err != nil {
		return nil, err
	}

	s.messagePublisher.PublishMessage(ctx, foundConversation, authID, *sentMessage)

	return sentMessage, nil
}

func (s *Service) List(ctx context.Context, authID string) ([]readmodel.ConversationSummary, error) {
	if consumerID, err := s.consumerIDFinder.FindIDByAuthID(authID); err == nil {
		return s.conversationReader.FindSummariesByConsumerID(ctx, consumerID)
	}

	if providerID, err := s.providerIDFinder.FindIDByAuthID(authID); err == nil {
		return s.conversationReader.FindSummariesByProviderID(ctx, providerID)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) CreateChatbotConversation(ctx context.Context, authID string, content string) (Conversation, error) {
	if s.chatbot == nil {
		return nil, ErrChatbotUnavailable
	}

	consumerID, err := s.consumerIDFinder.FindIDByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyConsumerCanMessageChatbot
	}

	consumerMessage, err := NewConsumerMessage(content)
	if err != nil {
		return nil, err
	}
	if !isHomeProblemQuestion(consumerMessage.Content) {
		return nil, ErrChatbotQuestionOutOfScope
	}

	chatbotResponse, err := s.chatbot.GetResponse(ctx, consumerMessage.Content)
	if err != nil {
		return nil, err
	}
	if chatbotResponse == nil {
		return nil, ErrChatbotResponseRequired
	}
	chatbotMessage, err := NewChatbotMessage(chatbotResponse.Content)
	if err != nil {
		return nil, err
	}

	chatbotConversation, err := NewChatBotConversation(consumerID, chatbotResponse.Title)
	if err != nil {
		return nil, err
	}

	chatbotConversation.AddMessage(*consumerMessage)
	chatbotConversation.AddMessage(*chatbotMessage)

	savedConversation, err := s.conversationRepository.SaveConversation(ctx, chatbotConversation)
	if err != nil {
		return nil, err
	}

	s.messagePublisher.PublishChatbotMessage(ctx, savedConversation, *chatbotMessage)

	return savedConversation, nil
}

func isHomeProblemQuestion(content string) bool {
	normalizedContent := strings.ToLower(strings.TrimSpace(content))
	homeProblemKeywords := []string{
		"agua",
		"baño",
		"bacha",
		"calefacción",
		"canilla",
		"casa",
		"cocina",
		"electricidad",
		"gas",
		"hogar",
		"humedad",
		"luz",
		"mueble",
		"pared",
		"pileta",
		"pérdida",
		"plomer",
		"techo",
	}
	for _, keyword := range homeProblemKeywords {
		if strings.Contains(normalizedContent, keyword) {
			return true
		}
	}

	return false
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

func (s *Service) authenticatedConsumerMatches(authID string, consumerID int) bool {
	authenticatedConsumerID, err := s.consumerIDFinder.FindIDByAuthID(authID)
	return err == nil && authenticatedConsumerID == consumerID
}

func (s *Service) authenticatedProviderMatches(authID string, providerID int) bool {
	authenticatedProviderID, err := s.providerIDFinder.FindIDByAuthID(authID)
	return err == nil && authenticatedProviderID == providerID
}

func (s *Service) newMessageForAuthenticatedParticipant(authID string, foundConversation Conversation, content string) (*Message, error) {
	workConversation, ok := foundConversation.(*WorkConversation)
	if !ok {
		return nil, ErrConversationAccessDenied
	}

	if s.authenticatedConsumerMatches(authID, workConversation.ConsumerID) {
		return NewConsumerMessage(content)
	}

	if s.authenticatedProviderMatches(authID, workConversation.ProviderID) {
		return NewProviderMessage(content)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) ensureMessageAllowedInCurrentConversationState(ctx context.Context, foundConversation Conversation, message Message) error {
	if foundConversation.Base().Status != StatusPending {
		return nil
	}

	if message.SenderRole == SenderProvider {
		return ErrPendingConversationRequiresAcceptance
	}

	sentMessages, err := s.conversationRepository.CountMessagesBySenderRole(ctx, foundConversation.Base().ID, SenderConsumer)
	if err != nil {
		return err
	}
	if sentMessages >= PendingConsumerMessageLimit {
		return ErrPendingConversationMessageLimitReached
	}

	return nil
}

func newPendingWorkRequest(consumerID, providerID int, content string) (Conversation, *Message, error) {
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
