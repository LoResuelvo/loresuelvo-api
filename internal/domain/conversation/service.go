package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	domainclock "github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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
	fileService            FileService
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
	fileService FileService,
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
		detail, err = s.withCounterpartProfilePhotoURL(ctx, detail)
		if err != nil {
			return nil, err
		}
		return s.withMessageImageURLs(ctx, detail)
	}

	if s.authenticatedProviderMatches(authID, workConversation.ProviderID) {
		detail, err := s.conversationReader.FindDetailByIDRoleAndType(ctx, workConversation.Base().ID, SenderProvider, TypeWork)
		if err != nil {
			return nil, err
		}
		detail, err = s.withCounterpartProfilePhotoURL(ctx, detail)
		if err != nil {
			return nil, err
		}
		return s.withMessageImageURLs(ctx, detail)
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

	detail, err = s.withRecommendedProviders(ctx, detail)
	if err != nil {
		return nil, err
	}
	return s.withMessageImageURLs(ctx, detail)
}

func (s *Service) SendMessage(ctx context.Context, authID string, conversationID int, content string, imageFileIDs []string) (*Message, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	senderRole, err := s.senderRoleForAuthenticatedParticipant(authID, foundConversation)
	if err != nil {
		return nil, err
	}
	images, err := s.messageImagesForSender(ctx, authID, imageFileIDs)
	if err != nil {
		return nil, err
	}
	message, err := newParticipantMessage(senderRole, content, images)
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

func (s *Service) CreateChatbotConversation(ctx context.Context, authID string, content string, imageFileIDs ...[]string) (*ChatbotConversationResult, error) {
	consumerID, err := s.chatbotConsumerID(authID)
	if err != nil {
		return nil, err
	}

	messageImages, chatbotImages, err := s.chatbotImagesForSender(ctx, authID, optionalImageFileIDs(imageFileIDs))
	if err != nil {
		return nil, err
	}
	consumerMessage, err := NewConsumerMessage(content, messageImages...)
	if err != nil {
		return nil, err
	}

	answer, err := s.answerChatbotQuestion(ctx, ChatbotHomeProblemQuestion{UserMessage: consumerMessage.Content, Images: chatbotImages, IsNewConversation: true})
	if err != nil {
		return nil, err
	}
	if err := applyChatbotImageDescriptions(consumerMessage, answer.response.ImageDescriptions); err != nil {
		return nil, err
	}
	selectedImages, err := selectedAssessmentImages(answer.response.Assessment, consumerMessage.Images)
	if err != nil {
		return nil, err
	}

	createdConversation, err := NewChatbotConversation(consumerID, answer.response.Title)
	if err != nil {
		return nil, err
	}
	chatbotConversation := createdConversation.(*ChatBotConversation)
	if err := chatbotConversation.ApplyResponse(*answer.response, problemCategoryID(answer.problemCategory), selectedImages...); err != nil {
		return nil, err
	}
	chatbotConversation.AddMessage(*consumerMessage)
	chatbotConversation.AddMessage(*answer.message)

	savedConversation, err := s.conversationRepository.SaveConversation(ctx, chatbotConversation)
	if err != nil {
		return nil, err
	}

	return chatbotTurnResult(savedConversation, answer), nil
}

func (s *Service) ContinueChatbotConversation(ctx context.Context, authID string, conversationID int, content string, imageFileIDs ...[]string) (*ChatbotConversationTurnResult, error) {
	consumerID, err := s.chatbotConsumerID(authID)
	if err != nil {
		return nil, err
	}

	messageImages, chatbotImages, err := s.chatbotImagesForSender(ctx, authID, optionalImageFileIDs(imageFileIDs))
	if err != nil {
		return nil, err
	}
	consumerMessage, err := NewConsumerMessage(content, messageImages...)
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

	question := chatbotHomeProblemQuestionFrom(chatbotConversation, consumerMessage.Content, chatbotImages)
	answer, err := s.answerChatbotQuestion(ctx, question)
	if err != nil {
		return nil, err
	}
	if err := applyChatbotImageDescriptions(consumerMessage, answer.response.ImageDescriptions); err != nil {
		return nil, err
	}
	availableImages := append(chatbotConversationImageEvidence(chatbotConversation), consumerMessage.Images...)
	selectedImages, err := selectedAssessmentImages(answer.response.Assessment, availableImages)
	if err != nil {
		return nil, err
	}
	if answer.response.Assessment.Action == ChatbotAssessmentUnchanged {
		answer.problemCategory, answer.recommendedProviders, err = s.recommendationForCurrentAssessment(ctx, chatbotConversation)
		if err != nil {
			return nil, err
		}
	}

	savedConversation, err := s.saveChatbotTurn(ctx, chatbotConversation, *consumerMessage, *answer.message, answer, selectedImages)
	if err != nil {
		return nil, err
	}
	processingFinished = true

	return chatbotTurnResult(savedConversation, answer), nil
}

func (s *Service) recommendationForCurrentAssessment(ctx context.Context, chatbotConversation *ChatBotConversation) (*category.Category, []providerreadmodel.ProviderSummary, error) {
	assessment := chatbotConversation.CurrentAssessment
	if assessment == nil || assessment.ProblemCategoryID == nil {
		return nil, nil, nil
	}
	categories, err := s.availableCategoriesForChatbot()
	if err != nil {
		return nil, nil, err
	}
	var matched *category.Category
	for i := range categories {
		if categories[i].ID == *assessment.ProblemCategoryID {
			matched = &categories[i]
			break
		}
	}
	if matched == nil || !assessment.RequiresProfessional() {
		return matched, nil, nil
	}
	providers, err := s.providerRepository.FindByCategoryID(matched.ID)
	if err != nil {
		return nil, nil, err
	}
	summaries, err := s.providerSummariesWithProfilePhotoURLs(ctx, providers)
	return matched, summaries, err
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

	problemCategory, recommendedProviders, err := s.assessmentResult(ctx, *chatbotResponse, availableCategories)
	if err != nil {
		return nil, err
	}

	return &chatbotAnswer{
		response:             chatbotResponse,
		message:              chatbotMessage,
		problemCategory:      problemCategory,
		recommendedProviders: recommendedProviders,
	}, nil
}

func problemCategoryID(problemCategory *category.Category) *int {
	if problemCategory == nil {
		return nil
	}

	categoryID := problemCategory.ID
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

func chatbotHomeProblemQuestionFrom(chatbotConversation *ChatBotConversation, userMessage string, images []filedomain.MessageImageContent) ChatbotHomeProblemQuestion {
	return ChatbotHomeProblemQuestion{
		UserMessage:    userMessage,
		ContextSummary: chatbotConversation.Context.Summary,
		RecentMessages: chatbotConversation.RecentMessages(ChatbotRecentMessageLimit),
		Images:         images,
	}
}

func (s *Service) saveChatbotTurn(ctx context.Context, chatbotConversation *ChatBotConversation, consumerMessage Message, chatbotMessage Message, answer *chatbotAnswer, selectedImages []filedomain.MessageImage) (Conversation, error) {
	if err := chatbotConversation.AddTurn(consumerMessage, chatbotMessage); err != nil {
		return nil, err
	}
	if err := chatbotConversation.ApplyResponse(*answer.response, problemCategoryID(answer.problemCategory), selectedImages...); err != nil {
		return nil, err
	}
	chatbotConversation.FinishProcessing()

	return s.conversationRepository.UpdateConversation(ctx, chatbotConversation)
}

func applyChatbotImageDescriptions(message *Message, descriptions []ChatbotImageDescription) error {
	if len(message.Images) != len(descriptions) {
		return ErrChatbotResponseRequired
	}
	byRef := make(map[string]string, len(descriptions))
	for _, description := range descriptions {
		ref := strings.TrimSpace(description.ImageRef)
		text := strings.TrimSpace(description.Description)
		if ref == "" || text == "" {
			return ErrChatbotResponseRequired
		}
		if _, exists := byRef[ref]; exists {
			return ErrChatbotResponseRequired
		}
		byRef[ref] = text
	}
	for index := range message.Images {
		description, exists := byRef[ChatbotImageRef(message.Images[index].FileID)]
		if !exists {
			return ErrChatbotResponseRequired
		}
		message.Images[index].Description = description
	}
	return nil
}

func selectedAssessmentImages(assessment ChatbotAssessmentResponse, available []filedomain.MessageImage) ([]filedomain.MessageImage, error) {
	if assessment.Action == ChatbotAssessmentUnchanged {
		if len(assessment.SelectedImageRefs) != 0 {
			return nil, ErrProblemAssessmentInvalid
		}
		return nil, nil
	}
	if len(assessment.SelectedImageRefs) > MaxProblemAssessmentImages {
		return nil, ErrProblemAssessmentInvalid
	}
	byRef := make(map[string]filedomain.MessageImage, len(available))
	for _, image := range available {
		if strings.TrimSpace(image.Description) != "" {
			byRef[ChatbotImageRef(image.FileID)] = image
		}
	}
	selected := make([]filedomain.MessageImage, 0, len(assessment.SelectedImageRefs))
	seen := make(map[string]struct{}, len(assessment.SelectedImageRefs))
	for _, rawRef := range assessment.SelectedImageRefs {
		ref := strings.TrimSpace(rawRef)
		if _, duplicate := seen[ref]; duplicate {
			return nil, ErrProblemAssessmentInvalid
		}
		image, exists := byRef[ref]
		if !exists {
			return nil, ErrProblemAssessmentInvalid
		}
		seen[ref] = struct{}{}
		selected = append(selected, image)
	}
	return selected, nil
}

func chatbotConversationImageEvidence(chatbotConversation *ChatBotConversation) []filedomain.MessageImage {
	var images []filedomain.MessageImage
	for _, message := range chatbotConversation.Messages() {
		if message.SenderRole == SenderConsumer {
			images = append(images, message.Images...)
		}
	}
	return images
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
		Assessment:           copyCurrentAssessment(conversation),
		ProblemCategory:      answer.problemCategory,
	}
}

func copyCurrentAssessment(foundConversation Conversation) *ProblemAssessment {
	chatbotConversation, ok := foundConversation.(*ChatBotConversation)
	if !ok || chatbotConversation.CurrentAssessment == nil {
		return nil
	}
	assessment := *chatbotConversation.CurrentAssessment
	assessment.ProblemCategoryID = copyOptionalInt(assessment.ProblemCategoryID)
	return &assessment
}

func (s *Service) authenticatedConsumerMatches(authID string, consumerID int) bool {
	authenticatedConsumerID, err := s.consumerRepository.FindIDByAuthID(authID)
	return err == nil && authenticatedConsumerID == consumerID
}

func (s *Service) authenticatedProviderMatches(authID string, providerID int) bool {
	authenticatedProviderID, err := s.providerRepository.FindIDByAuthID(authID)
	return err == nil && authenticatedProviderID == providerID
}

func (s *Service) senderRoleForAuthenticatedParticipant(authID string, foundConversation Conversation) (string, error) {
	workConversation, ok := foundConversation.(*WorkConversation)
	if !ok {
		return "", ErrConversationAccessDenied
	}

	if s.authenticatedConsumerMatches(authID, workConversation.ConsumerID) {
		return SenderConsumer, nil
	}

	if s.authenticatedProviderMatches(authID, workConversation.ProviderID) {
		return SenderProvider, nil
	}

	return "", ErrConversationAccessDenied
}

func newParticipantMessage(senderRole, content string, images []filedomain.MessageImage) (*Message, error) {
	if senderRole == SenderConsumer {
		return NewConsumerMessage(content, images...)
	}
	return NewProviderMessage(content, images...)
}

func (s *Service) messageImagesForSender(ctx context.Context, authID string, fileIDs []string) ([]filedomain.MessageImage, error) {
	preparedFiles, err := s.fileService.PrepareMessageImages(ctx, authID, fileIDs)
	if err != nil {
		if errors.Is(err, filedomain.ErrMessageImageNotAvailable) {
			return nil, ErrMessageImageNotAvailable
		}
		return nil, fmt.Errorf("preparing message images: %w", err)
	}
	return preparedFiles, nil
}

func (s *Service) chatbotImagesForSender(ctx context.Context, authID string, fileIDs []string) ([]filedomain.MessageImage, []filedomain.MessageImageContent, error) {
	preparedFiles, err := s.fileService.PrepareChatbotMessageImages(ctx, authID, fileIDs)
	if err != nil {
		if errors.Is(err, filedomain.ErrMessageImageNotAvailable) {
			return nil, nil, ErrMessageImageNotAvailable
		}
		return nil, nil, fmt.Errorf("preparing chatbot message images: %w", err)
	}

	messageImages := make([]filedomain.MessageImage, 0, len(preparedFiles))
	chatbotImages := make([]filedomain.MessageImageContent, 0, len(preparedFiles))
	for _, file := range preparedFiles {
		messageImages = append(messageImages, file.MessageImage)
		chatbotImages = append(chatbotImages, file)
	}
	return messageImages, chatbotImages, nil
}

func optionalImageFileIDs(imageFileIDs [][]string) []string {
	if len(imageFileIDs) == 0 {
		return nil
	}
	return imageFileIDs[0]
}

func (s *Service) withMessageImageURLs(ctx context.Context, detail *readmodel.ConversationDetail) (*readmodel.ConversationDetail, error) {
	fileIDs := make([]string, 0)
	for _, message := range detail.Messages {
		for _, image := range message.Images {
			fileIDs = append(fileIDs, image.FileID)
		}
	}
	resolved, err := s.fileService.ResolveMessageImages(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation message images: %w", err)
	}
	for messageIndex := range detail.Messages {
		for imageIndex := range detail.Messages[messageIndex].Images {
			image := &detail.Messages[messageIndex].Images[imageIndex]
			file, ok := resolved[image.FileID]
			if !ok || file.URL == "" {
				return nil, ErrMessageImageNotAvailable
			}
			image.OriginalName = file.OriginalName
			image.URL = file.URL
		}
	}
	return detail, nil
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
func (s *Service) assessmentResult(ctx context.Context, chatbotResponse ChatbotResponse, availableCategories []category.Category) (*category.Category, []providerreadmodel.ProviderSummary, error) {
	if chatbotResponse.Assessment.Action == ChatbotAssessmentUnchanged {
		return nil, nil, nil
	}

	matchedCategory := categoryForChatbotResponse(chatbotResponse, availableCategories)
	if strings.TrimSpace(chatbotResponse.Assessment.ProblemCategoryName) != "" && matchedCategory == nil {
		return nil, nil, ErrProblemAssessmentInvalid
	}
	if chatbotResponse.Assessment.Outcome == AssessmentProfessionalRequired && matchedCategory == nil {
		return nil, nil, ErrProblemAssessmentInvalid
	}
	if chatbotResponse.Assessment.Outcome != AssessmentProfessionalRequired {
		return matchedCategory, nil, nil
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
	categoryName := strings.TrimSpace(chatbotResponse.Assessment.ProblemCategoryName)
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
	if detail.Chatbot == nil || detail.Chatbot.Assessment == nil || detail.Chatbot.Assessment.Outcome != string(AssessmentProfessionalRequired) || detail.Chatbot.Assessment.ProblemCategory == nil {
		return detail, nil
	}

	providers, err := s.providerRepository.FindByCategoryID(detail.Chatbot.Assessment.ProblemCategory.ID)
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
