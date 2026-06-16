package conversation

import (
	"context"
	"fmt"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
)

type Service struct {
	conversationRepository Repository
	consumerRepository     ConsumerIDFinder
	providerRepository     ProviderIDFinder
	conversationReader     Reader
	messagePublisher       MessagePublisher
	chatbot                Chatbot
	fileService            FileURLResolver
}

func NewService(
	conversationRepository Repository,
	consumerRepository ConsumerIDFinder,
	providerRepository ProviderIDFinder,
	conversationReader Reader,
	messagePublisher MessagePublisher,
	chatbot Chatbot,
	fileService FileURLResolver,
) *Service {
	return &Service{
		conversationRepository: conversationRepository,
		consumerRepository:     consumerRepository,
		providerRepository:     providerRepository,
		conversationReader:     conversationReader,
		messagePublisher:       messagePublisher,
		chatbot:                chatbot,
		fileService:            fileService,
	}
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
		detail, err := s.conversationReader.FindDetailByIDForConsumer(ctx, conversationID)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURL(ctx, detail)
	}

	if s.authenticatedProviderMatches(authID, workConversation.ProviderID) {
		detail, err := s.conversationReader.FindDetailByIDForProvider(ctx, conversationID)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURL(ctx, detail)
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
	if consumerID, err := s.consumerRepository.FindIDByAuthID(authID); err == nil {
		summaries, err := s.conversationReader.FindSummariesByConsumerID(ctx, consumerID)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURLs(ctx, summaries)
	}

	if providerID, err := s.providerRepository.FindIDByAuthID(authID); err == nil {
		summaries, err := s.conversationReader.FindSummariesByProviderID(ctx, providerID)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURLs(ctx, summaries)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) CreateChatbotConversation(ctx context.Context, authID string, content string) (*ChatbotConversationResult, error) {
	consumerID, err := s.consumerRepository.FindIDByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyConsumerCanMessageChatbot
	}

	consumerMessage, err := NewConsumerMessage(content)
	if err != nil {
		return nil, err
	}

	chatbotResponse, err := s.chatbot.AnswerHomeProblemQuestion(ctx, consumerMessage.Content)
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

	chatbotConversation, err := NewChatbotConversation(consumerID, chatbotResponse.Title)
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

	return &ChatbotConversationResult{
		Conversation:   savedConversation,
		ResponseStatus: chatbotResponse.Status,
	}, nil
}

func (s *Service) authenticatedConsumerMatches(authID string, consumerID int) bool {
	authenticatedConsumerID, err := s.consumerRepository.FindIDByAuthID(authID)
	return err == nil && authenticatedConsumerID == consumerID
}

func (s *Service) authenticatedProviderMatches(authID string, providerID int) bool {
	authenticatedProviderID, err := s.providerRepository.FindIDByAuthID(authID)
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

func (s *Service) withCounterpartProfilePhotoURLs(ctx context.Context, summaries []readmodel.ConversationSummary) ([]readmodel.ConversationSummary, error) {
	fileIDs := make([]string, 0, len(summaries))
	for i := range summaries {
		fileIDs = append(fileIDs, summaries[i].Counterpart.ProfilePhotoFileID)
	}
	urlsByFileID, err := s.fileService.ResolvePublicURLs(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation counterpart profile photo urls: %w", err)
	}
	for i := range summaries {
		summaries[i].Counterpart.ProfilePhotoURL = urlsByFileID[summaries[i].Counterpart.ProfilePhotoFileID]
	}

	return summaries, nil
}

func (s *Service) withCounterpartProfilePhotoURL(ctx context.Context, detail *readmodel.ConversationDetail) (*readmodel.ConversationDetail, error) {
	profilePhotoURL, err := s.fileService.ResolvePublicURL(ctx, detail.Counterpart.ProfilePhotoFileID)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation counterpart profile photo url: %w", err)
	}
	detail.Counterpart.ProfilePhotoURL = profilePhotoURL

	return detail, nil
}
