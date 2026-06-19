package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	domainclock "github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type Service struct {
	conversationRepository Repository
	consumerRepository     ConsumerIDFinder
	providerRepository     ProviderRepository
	conversationReader     Reader
	messagePublisher       MessagePublisher
	chatbot                Chatbot
	categoryRepository     RecommendationCategoryLister
	fileService            FileURLResolver
	clock                  domainclock.Clock
}

func NewService(
	conversationRepository Repository,
	consumerRepository ConsumerIDFinder,
	providerRepository ProviderRepository,
	conversationReader Reader,
	messagePublisher MessagePublisher,
	chatbot Chatbot,
	categoryRepository RecommendationCategoryLister,
	fileService FileURLResolver,
	clock domainclock.Clock,
) *Service {
	return &Service{
		conversationRepository: conversationRepository,
		consumerRepository:     consumerRepository,
		providerRepository:     providerRepository,
		conversationReader:     conversationReader,
		messagePublisher:       messagePublisher,
		chatbot:                chatbot,
		categoryRepository:     categoryRepository,
		fileService:            fileService,
		clock:                  clock,
	}
}

func (s *Service) GetByID(ctx context.Context, authID string, conversationID int) (*readmodel.ConversationDetail, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	switch typedConversation := foundConversation.(type) {
	case *WorkConversation:
		return s.getWorkConversationDetail(ctx, authID, typedConversation)
	case *ChatBotConversation:
		return s.getChatbotConversationDetail(ctx, authID, typedConversation)
	default:
		return nil, ErrConversationAccessDenied
	}
}

func (s *Service) getWorkConversationDetail(ctx context.Context, authID string, workConversation *WorkConversation) (*readmodel.ConversationDetail, error) {
	if s.authenticatedConsumerMatches(authID, workConversation.ConsumerID) {
		detail, err := s.conversationReader.FindDetailByIDRoleAndType(ctx, workConversation.Base().ID, SenderConsumer, TypeWork)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURL(ctx, detail)
	}

	if s.authenticatedProviderMatches(authID, workConversation.ProviderID) {
		detail, err := s.conversationReader.FindDetailByIDRoleAndType(ctx, workConversation.Base().ID, SenderProvider, TypeWork)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURL(ctx, detail)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) getChatbotConversationDetail(ctx context.Context, authID string, chatbotConversation *ChatBotConversation) (*readmodel.ConversationDetail, error) {
	if !s.authenticatedConsumerMatches(authID, chatbotConversation.ConsumerID) {
		return nil, ErrConversationAccessDenied
	}

	detail, err := s.conversationReader.FindDetailByIDRoleAndType(ctx, chatbotConversation.Base().ID, SenderConsumer, TypeChatbot)
	if err != nil {
		return nil, err
	}

	return s.withRecommendedProviders(ctx, detail)
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

func (s *Service) ListWorkConversations(ctx context.Context, authID string) ([]readmodel.ConversationSummary, error) {
	if consumerID, err := s.consumerRepository.FindIDByAuthID(authID); err == nil {
		summaries, err := s.conversationReader.FindSummariesByParticipantIDRoleAndType(ctx, consumerID, SenderConsumer, TypeWork)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURLs(ctx, summaries)
	}

	if providerID, err := s.providerRepository.FindIDByAuthID(authID); err == nil {
		summaries, err := s.conversationReader.FindSummariesByParticipantIDRoleAndType(ctx, providerID, SenderProvider, TypeWork)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURLs(ctx, summaries)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) ListChatbotConversations(ctx context.Context, authID string) ([]readmodel.ConversationSummary, error) {
	consumerID, err := s.consumerRepository.FindIDByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyConsumerCanListChatbotConversations
	}

	return s.conversationReader.FindSummariesByParticipantIDRoleAndType(ctx, consumerID, SenderConsumer, TypeChatbot)
}

func (s *Service) CreateChatbotConversation(ctx context.Context, authID string, content string) (*ChatbotConversationResult, error) {
	consumerID, err := s.chatbotConsumerID(authID)
	if err != nil {
		return nil, err
	}

	consumerMessage, err := NewConsumerMessage(content)
	if err != nil {
		return nil, err
	}

	answer, err := s.answerChatbotQuestion(ctx, ChatbotHomeProblemQuestion{UserMessage: consumerMessage.Content})
	if err != nil {
		return nil, err
	}

	createdConversation, err := NewChatbotConversation(consumerID, answer.response.Title)
	if err != nil {
		return nil, err
	}
	chatbotConversation := createdConversation.(*ChatBotConversation)
	chatbotConversation.ApplyResponse(*answer.response, recommendedCategoryID(answer.recommendedCategory))
	chatbotConversation.AddMessage(*consumerMessage)
	chatbotConversation.AddMessage(*answer.message)

	savedConversation, err := s.conversationRepository.SaveConversation(ctx, chatbotConversation)
	if err != nil {
		return nil, err
	}

	return chatbotTurnResult(savedConversation, answer), nil
}

func (s *Service) ContinueChatbotConversation(ctx context.Context, authID string, conversationID int, content string) (*ChatbotConversationTurnResult, error) {
	consumerID, err := s.chatbotConsumerID(authID)
	if err != nil {
		return nil, err
	}

	consumerMessage, err := NewConsumerMessage(content)
	if err != nil {
		return nil, err
	}

	chatbotConversation, err := s.findOwnedChatbotConversation(ctx, conversationID, consumerID)
	if err != nil {
		return nil, err
	}

	if err := s.startChatbotProcessing(ctx, chatbotConversation); err != nil {
		return nil, err
	}
	processingFinished := false
	defer func() {
		if !processingFinished {
			s.finishChatbotProcessing(context.WithoutCancel(ctx), chatbotConversation)
		}
	}()

	if err := s.refreshChatbotContextIfNeeded(ctx, chatbotConversation); err != nil {
		return nil, err
	}

	question := chatbotHomeProblemQuestionFrom(chatbotConversation, consumerMessage.Content)
	answer, err := s.answerChatbotQuestion(ctx, question)
	if err != nil {
		return nil, err
	}

	savedConversation, err := s.saveChatbotTurn(ctx, chatbotConversation, *consumerMessage, *answer.message, answer)
	if err != nil {
		return nil, err
	}
	processingFinished = true

	return chatbotTurnResult(savedConversation, answer), nil
}

func (s *Service) chatbotConsumerID(authID string) (int, error) {
	consumerID, err := s.consumerRepository.FindIDByAuthID(authID)
	if err != nil {
		return 0, ErrOnlyConsumerCanMessageChatbot
	}

	return consumerID, nil
}

func (s *Service) findOwnedChatbotConversation(ctx context.Context, conversationID int, consumerID int) (*ChatBotConversation, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	chatbotConversation, ok := foundConversation.(*ChatBotConversation)
	if !ok {
		return nil, ErrConversationDoesNotExist
	}
	if chatbotConversation.ConsumerID != consumerID {
		return nil, ErrConversationAccessDenied
	}

	return chatbotConversation, nil
}

func (s *Service) answerChatbotQuestion(ctx context.Context, question ChatbotHomeProblemQuestion) (*chatbotAnswer, error) {
	availableCategories, err := s.availableCategoriesForChatbot()
	if err != nil {
		return nil, err
	}

	chatbotResponse, err := s.chatbot.AnswerHomeProblemQuestion(ctx, question, availableCategories)
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

	recommendedCategory, recommendedProviders, err := s.recommendationForChatbotResponse(ctx, *chatbotResponse, availableCategories)
	if err != nil {
		return nil, err
	}

	return &chatbotAnswer{
		response:             chatbotResponse,
		message:              chatbotMessage,
		recommendedCategory:  recommendedCategory,
		recommendedProviders: recommendedProviders,
	}, nil
}

func recommendedCategoryID(recommendedCategory *category.Category) *int {
	if recommendedCategory == nil {
		return nil
	}

	categoryID := recommendedCategory.ID
	return &categoryID
}

func (s *Service) startChatbotProcessing(ctx context.Context, chatbotConversation *ChatBotConversation) error {
	if err := chatbotConversation.StartProcessing(s.clock.Now()); err != nil {
		return err
	}

	_, err := s.conversationRepository.UpdateConversation(ctx, chatbotConversation)
	return err
}

func (s *Service) finishChatbotProcessing(ctx context.Context, chatbotConversation *ChatBotConversation) {
	chatbotConversation.FinishProcessing()
	_, _ = s.conversationRepository.UpdateConversation(ctx, chatbotConversation)
}

func (s *Service) refreshChatbotContextIfNeeded(ctx context.Context, chatbotConversation *ChatBotConversation) error {
	if !chatbotConversation.ShouldSummarizeContext(ChatbotRecentMessageLimit) {
		return nil
	}

	pendingMessages := chatbotConversation.MessagesPendingSummary()
	contextSummary, err := s.summarizeChatbotContext(ctx, chatbotConversation.Context.Summary, pendingMessages)
	if err != nil {
		return err
	}

	lastPendingMessage := pendingMessages[len(pendingMessages)-1]
	if err := chatbotConversation.UpdateContext(ChatbotConversationContext{
		Summary:                 contextSummary,
		LastSummarizedMessageID: lastPendingMessage.ID,
	}); err != nil {
		return err
	}

	_, err = s.conversationRepository.UpdateConversation(ctx, chatbotConversation)
	return err
}

func chatbotHomeProblemQuestionFrom(chatbotConversation *ChatBotConversation, userMessage string) ChatbotHomeProblemQuestion {
	return ChatbotHomeProblemQuestion{
		UserMessage:    userMessage,
		ContextSummary: chatbotConversation.Context.Summary,
		RecentMessages: chatbotConversation.RecentMessages(ChatbotRecentMessageLimit),
	}
}

func (s *Service) saveChatbotTurn(ctx context.Context, chatbotConversation *ChatBotConversation, consumerMessage Message, chatbotMessage Message, answer *chatbotAnswer) (Conversation, error) {
	if err := chatbotConversation.AddTurn(consumerMessage, chatbotMessage); err != nil {
		return nil, err
	}
	chatbotConversation.ApplyResponse(*answer.response, recommendedCategoryID(answer.recommendedCategory))
	chatbotConversation.FinishProcessing()

	return s.conversationRepository.UpdateConversation(ctx, chatbotConversation)
}

func (s *Service) summarizeChatbotContext(ctx context.Context, previousSummary string, messages []Message) (string, error) {
	contextSummary, err := s.chatbot.SummarizeHomeProblemConversation(ctx, previousSummary, messages)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(contextSummary), nil
}

func chatbotTurnResult(conversation Conversation, answer *chatbotAnswer) *ChatbotConversationTurnResult {
	return &ChatbotConversationTurnResult{
		Conversation:         conversation,
		ResponseStatus:       answer.response.Status,
		RecommendedProviders: answer.recommendedProviders,
		DiagnosisCompleted:   answer.response.DiagnosisCompleted,
		RecommendedCategory:  answer.recommendedCategory,
	}
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
		if summaries[i].Work == nil {
			continue
		}
		fileIDs = append(fileIDs, summaries[i].Work.Counterpart.ProfilePhotoFileID)
	}
	urlsByFileID, err := s.fileService.ResolvePublicURLs(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation counterpart profile photo urls: %w", err)
	}
	for i := range summaries {
		if summaries[i].Work == nil {
			continue
		}
		summaries[i].Work.Counterpart.ProfilePhotoURL = urlsByFileID[summaries[i].Work.Counterpart.ProfilePhotoFileID]
	}

	return summaries, nil
}

func (s *Service) withCounterpartProfilePhotoURL(ctx context.Context, detail *readmodel.ConversationDetail) (*readmodel.ConversationDetail, error) {
	if detail.Work == nil {
		return detail, nil
	}

	profilePhotoURL, err := s.fileService.ResolvePublicURL(ctx, detail.Work.Counterpart.ProfilePhotoFileID)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation counterpart profile photo url: %w", err)
	}
	detail.Work.Counterpart.ProfilePhotoURL = profilePhotoURL

	return detail, nil
}

// TODO: Let the chatbot rank providers using richer provider attributes when recommendation criteria evolve.
func (s *Service) recommendationForChatbotResponse(ctx context.Context, chatbotResponse ChatbotResponse, availableCategories []category.Category) (*category.Category, []providerreadmodel.ProviderSummary, error) {
	if !chatbotResponse.DiagnosisCompleted {
		return nil, nil, nil
	}

	matchedCategory := categoryForChatbotResponse(chatbotResponse, availableCategories)
	if matchedCategory == nil {
		return nil, []providerreadmodel.ProviderSummary{}, nil
	}

	providers, err := s.providerRepository.FindByCategoryID(matchedCategory.ID)
	if err != nil {
		return nil, nil, err
	}

	summaries, err := s.providerSummariesWithProfilePhotoURLs(ctx, providers)
	if err != nil {
		return nil, nil, err
	}

	return matchedCategory, summaries, nil
}

func categoryForChatbotResponse(chatbotResponse ChatbotResponse, availableCategories []category.Category) *category.Category {
	categoryName := strings.TrimSpace(chatbotResponse.RecommendedCategoryName)
	if categoryName == "" {
		return nil
	}

	recommendedCategory, err := category.New(categoryName)
	if err != nil {
		return nil
	}

	return findCategoryByNormalizedName(availableCategories, recommendedCategory.NormalizedName)
}

func (s *Service) withRecommendedProviders(ctx context.Context, detail *readmodel.ConversationDetail) (*readmodel.ConversationDetail, error) {
	if detail.Chatbot == nil || !detail.Chatbot.DiagnosisCompleted || detail.Chatbot.RecommendedCategory == nil || detail.Chatbot.RecommendedCategory.ID == 0 {
		return detail, nil
	}

	providers, err := s.providerRepository.FindByCategoryID(detail.Chatbot.RecommendedCategory.ID)
	if err != nil {
		return nil, err
	}

	detail.Chatbot.RecommendedProviders, err = s.providerSummariesWithProfilePhotoURLs(ctx, providers)
	if err != nil {
		return nil, err
	}

	return detail, nil
}

func (s *Service) providerSummariesWithProfilePhotoURLs(ctx context.Context, providers []provider.Provider) ([]providerreadmodel.ProviderSummary, error) {
	profilePhotoFileIDs := make([]string, 0, len(providers))
	for i := range providers {
		profilePhotoFileIDs = append(profilePhotoFileIDs, providers[i].ProfilePhotoFileID)
	}

	profilePhotoURLs, err := s.fileService.ResolvePublicURLs(ctx, profilePhotoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving chatbot recommended provider profile photo urls: %w", err)
	}

	return provider.SummariesWithProfilePhotoURLs(providers, profilePhotoURLs), nil
}

func (s *Service) availableCategoriesForChatbot() ([]category.Category, error) {
	categories, err := s.categoryRepository.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing categories for chatbot prompt: %w", err)
	}

	return categories, nil
}

func findCategoryByNormalizedName(categories []category.Category, normalizedName string) *category.Category {
	for i := range categories {
		if categories[i].NormalizedName == normalizedName {
			return &categories[i]
		}
	}

	return nil
}
