package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	domainclock "github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type Service struct {
	conversationRepository Repository
	userRepository         UserRepository
	conversationReader     Reader
	messagePublisher       MessagePublisher
	chatbot                Chatbot
	categoryRepository     RecommendationCategoryLister
	fileService            FileService
	clock                  domainclock.Clock
	workOrderReader        WorkOrderReader
	recommendationConfig   ProviderRecommendationConfig
}

func NewService(
	conversationRepository Repository,
	userRepository UserRepository,
	conversationReader Reader,
	messagePublisher MessagePublisher,
	chatbot Chatbot,
	categoryRepository RecommendationCategoryLister,
	fileService FileService,
	clock domainclock.Clock,
	recommendationConfig ProviderRecommendationConfig,
	workOrderReader WorkOrderReader,
) *Service {
	return &Service{
		conversationRepository: conversationRepository,
		userRepository:         userRepository,
		conversationReader:     conversationReader,
		messagePublisher:       messagePublisher,
		chatbot:                chatbot,
		categoryRepository:     categoryRepository,
		fileService:            fileService,
		clock:                  clock,
		workOrderReader:        workOrderReader,
		recommendationConfig:   recommendationConfig,
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
	authenticatedUser, err := s.authenticatedUserForConversation(authID, workConversation)
	if err != nil {
		return nil, err
	}

	detail, err := s.conversationReader.FindDetailByIDRoleAndType(
		ctx,
		workConversation.ID(),
		authenticatedUser.Role(),
		workConversation.ConversationType(),
	)
	if err != nil {
		return nil, err
	}
	detail, err = s.withCounterpartProfilePhotoURL(ctx, detail)
	if err != nil {
		return nil, err
	}
	return s.withMessageAttachmentURLs(ctx, detail)
}

func (s *Service) getChatbotConversationDetail(ctx context.Context, authID string, chatbotConversation *ChatBotConversation) (*readmodel.ConversationDetail, error) {
	authenticatedConsumer, err := s.authenticatedConsumerForChatbot(ctx, authID, chatbotConversation.ConsumerID)
	if err != nil {
		return nil, err
	}

	detail, err := s.conversationReader.FindDetailByIDRoleAndType(ctx, chatbotConversation.ID(), SenderConsumer, TypeChatbot)
	if err != nil {
		return nil, err
	}

	if chatbotConversation.CurrentRecommendation != nil && detail.Chatbot != nil {
		detail.Chatbot.RecommendationReasons = recommendationReasonsForCurrentRecommendation(chatbotConversation.CurrentRecommendation)
		detail.Chatbot.RecommendedProviders, err = s.providersForCurrentRecommendation(ctx, chatbotConversation.CurrentRecommendation)
	} else {
		detail, err = s.withRecommendedProviders(ctx, detail, authenticatedConsumer.CoverageZone().ID)
	}
	if err != nil {
		return nil, err
	}
	return s.withMessageAttachmentURLs(ctx, detail)
}

func (s *Service) SendMessage(ctx context.Context, authID string, conversationID int, content string, imageFileIDs []string, audioFileID string, videoFileIDs ...string) (*Message, error) {
	content = strings.TrimSpace(content)
	audioFileID = strings.TrimSpace(audioFileID)
	imageFileIDs = normalizeMessageImageFileIDs(imageFileIDs)
	videoFileID := ""
	if len(videoFileIDs) > 0 {
		videoFileID = strings.TrimSpace(videoFileIDs[0])
	}
	if audioFileID != "" && (content != "" || len(imageFileIDs) > 0 || videoFileID != "") {
		return nil, ErrMessageAudioMustBeExclusive
	}
	if videoFileID != "" && len(imageFileIDs) > 0 {
		return nil, ErrMessageVideoCannotIncludeImages
	}

	return s.sendParticipantMessage(ctx, authID, conversationID, content, imageFileIDs, audioFileID, videoFileID)
}

func (s *Service) sendParticipantMessage(ctx context.Context, authID string, conversationID int, content string, imageFileIDs []string, audioFileID string, videoFileID string) (*Message, error) {
	foundConversation, err := s.conversationRepository.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	senderRole, err := s.senderRoleForAuthenticatedParticipant(authID, foundConversation)
	if err != nil {
		return nil, err
	}
	var message *Message
	if audioFileID != "" {
		var audio *filedomain.MessageAudio
		audio, err = s.messageAudioForSender(ctx, authID, audioFileID)
		if err != nil {
			return nil, err
		}
		message, err = newParticipantAudioMessage(senderRole, *audio)
	} else if videoFileID != "" {
		var video *filedomain.MessageVideo
		video, err = s.messageVideoForSender(ctx, authID, videoFileID)
		if err != nil {
			return nil, err
		}
		message, err = newParticipantVideoMessage(senderRole, content, *video)
	} else {
		var images []filedomain.MessageImage
		images, err = s.messageImagesForSender(ctx, authID, imageFileIDs)
		if err != nil {
			return nil, err
		}
		message, err = newParticipantMessage(senderRole, content, images)
	}
	if err != nil {
		return nil, err
	}
	if err := s.ensureMessageAllowedInCurrentConversationState(ctx, foundConversation, *message); err != nil {
		return nil, err
	}
	foundConversation.AddMessage(*message)
	savedConversation, err := s.conversationRepository.SaveConversation(ctx, foundConversation)
	if err != nil {
		return nil, err
	}
	sentMessage, ok := savedConversation.LastMessage()
	if !ok {
		return nil, ErrConversationDoesNotExist
	}

	s.messagePublisher.PublishMessage(ctx, savedConversation, authID, sentMessage)

	return &sentMessage, nil
}

func normalizeMessageImageFileIDs(fileIDs []string) []string {
	if fileIDs == nil {
		return nil
	}

	normalized := make([]string, len(fileIDs))
	for index, fileID := range fileIDs {
		normalized[index] = strings.TrimSpace(fileID)
	}
	return normalized
}

func (s *Service) ListWorkConversations(ctx context.Context, authID string) ([]readmodel.ConversationSummary, error) {
	if user, err := s.userRepository.FindByAuthID(authID); err == nil {
		summaries, err := s.conversationReader.FindSummariesByUserAndType(ctx, user, TypeWork)
		if err != nil {
			return nil, err
		}
		return s.withCounterpartProfilePhotoURLs(ctx, summaries)
	}

	return nil, ErrConversationAccessDenied
}

func (s *Service) ListChatbotConversations(ctx context.Context, authID string) ([]readmodel.ConversationSummary, error) {
	foundConsumer, err := s.userRepository.FindConsumerByAuthID(ctx, authID)
	if err != nil || foundConsumer == nil {
		return nil, ErrOnlyConsumerCanListChatbotConversations
	}

	return s.conversationReader.FindSummariesByUserAndType(ctx, foundConsumer, TypeChatbot)
}

// TODO: recibir obligatoriamente una lista de strings para evitar el patrón optionalImageFIleIDs
func (s *Service) CreateChatbotConversation(ctx context.Context, authID string, content string, imageFileIDs ...[]string) (*ChatbotConversationResult, error) {
	chatbotConsumer, err := s.chatbotConsumer(ctx, authID)
	if err != nil {
		return nil, err
	}
	consumerID := chatbotConsumer.ID()
	coverageZoneID := chatbotConsumer.CoverageZone().ID

	messageImages, chatbotImages, err := s.chatbotImagesForSender(ctx, authID, optionalImageFileIDs(imageFileIDs))
	if err != nil {
		return nil, err
	}
	consumerMessage, err := NewConsumerMessage(content, messageImages...)
	if err != nil {
		return nil, err
	}

	answer, err := s.answerChatbotQuestion(ctx, ChatbotHomeProblemQuestion{UserMessage: consumerMessage.Content, Images: chatbotImages, IsNewConversation: true}, coverageZoneID)
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
	if answer.currentRecommendation != nil {
		if err := chatbotConversation.SetCurrentRecommendation(answer.currentRecommendation); err != nil {
			return nil, err
		}
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
	chatbotConsumer, err := s.chatbotConsumer(ctx, authID)
	if err != nil {
		return nil, err
	}
	consumerID := chatbotConsumer.ID()
	coverageZoneID := chatbotConsumer.CoverageZone().ID

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
	answer, err := s.answerChatbotQuestion(ctx, question, coverageZoneID)
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
		answer.problemCategory, answer.recommendedProviders, err = s.recommendationForCurrentAssessment(ctx, chatbotConversation, coverageZoneID)
		if err != nil {
			return nil, err
		}
		answer.recommendationReasons = recommendationReasonsForCurrentRecommendation(chatbotConversation.CurrentRecommendation)
	}

	savedConversation, err := s.saveChatbotTurn(ctx, chatbotConversation, *consumerMessage, *answer.message, answer, selectedImages)
	if err != nil {
		return nil, err
	}
	processingFinished = true

	return chatbotTurnResult(savedConversation, answer), nil
}

func (s *Service) recommendationForCurrentAssessment(ctx context.Context, chatbotConversation *ChatBotConversation, coverageZoneID int) (*category.Category, []provider.Provider, error) {
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
	if chatbotConversation.CurrentRecommendation != nil {
		providers, err := s.providersForCurrentRecommendation(ctx, chatbotConversation.CurrentRecommendation)
		return matched, providers, err
	}
	providers, err := s.userRepository.FindProvidersByCategoryAndCoverageZoneID(ctx, matched.ID, coverageZoneID)
	if err != nil {
		return nil, nil, err
	}
	summaries, err := s.providersWithProfilePhotoURLs(ctx, providers)
	return matched, summaries, err
}

func (s *Service) chatbotConsumer(ctx context.Context, authID string) (*consumer.Consumer, error) {
	foundConsumer, err := s.userRepository.FindConsumerByAuthID(ctx, authID)
	if err != nil || foundConsumer == nil {
		return nil, ErrOnlyConsumerCanMessageChatbot
	}

	return foundConsumer, nil
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

func (s *Service) answerChatbotQuestion(ctx context.Context, question ChatbotHomeProblemQuestion, coverageZoneID int) (*chatbotAnswer, error) {
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

	problemCategory, recommendedProviders, recommendationReasons, currentRecommendation, err := s.assessmentResult(ctx, *chatbotResponse, availableCategories, coverageZoneID)
	if err != nil {
		return nil, err
	}

	return &chatbotAnswer{
		response:              chatbotResponse,
		message:               chatbotMessage,
		problemCategory:       problemCategory,
		recommendedProviders:  recommendedProviders,
		recommendationReasons: recommendationReasons,
		currentRecommendation: currentRecommendation,
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

	_, err := s.conversationRepository.SaveConversation(ctx, chatbotConversation)
	return err
}

func (s *Service) finishChatbotProcessing(ctx context.Context, chatbotConversation *ChatBotConversation) {
	chatbotConversation.FinishProcessing()
	_, _ = s.conversationRepository.SaveConversation(ctx, chatbotConversation)
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

	_, err = s.conversationRepository.SaveConversation(ctx, chatbotConversation)
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
	if answer.currentRecommendation != nil {
		if err := chatbotConversation.SetCurrentRecommendation(answer.currentRecommendation); err != nil {
			return nil, err
		}
	}
	chatbotConversation.FinishProcessing()

	return s.conversationRepository.SaveConversation(ctx, chatbotConversation)
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
		Conversation:          conversation,
		ResponseStatus:        answer.response.Status,
		RecommendedProviders:  answer.recommendedProviders,
		RecommendationReasons: copyRecommendationReasons(answer.recommendationReasons),
		Assessment:            copyCurrentAssessment(conversation),
		ProblemCategory:       answer.problemCategory,
	}
}

func copyRecommendationReasons(reasons map[int]string) map[int]string {
	if len(reasons) == 0 {
		return nil
	}
	copied := make(map[int]string, len(reasons))
	for providerID, reason := range reasons {
		copied[providerID] = reason
	}
	return copied
}

func recommendationReasonsForCurrentRecommendation(currentRecommendation *CurrentProviderRecommendation) map[int]string {
	if currentRecommendation == nil {
		return nil
	}
	reasons := make(map[int]string, len(currentRecommendation.Recommendations))
	for _, recommendation := range currentRecommendation.Recommendations {
		reasons[recommendation.ProviderID] = recommendation.Reason
	}
	return reasons
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

func (s *Service) authenticatedConsumerForChatbot(ctx context.Context, authID string, consumerID int) (*consumer.Consumer, error) {
	authenticatedConsumer, err := s.userRepository.FindConsumerByAuthID(ctx, authID)
	if err != nil || authenticatedConsumer == nil || authenticatedConsumer.ID() != consumerID {
		return nil, ErrConversationAccessDenied
	}

	return authenticatedConsumer, nil
}

func (s *Service) authenticatedUserForConversation(authID string, conversation *WorkConversation) (user.User, error) {
	authenticatedUser, err := s.userRepository.FindByAuthID(authID)
	if err != nil {
		return nil, ErrConversationAccessDenied
	}
	role := authenticatedUser.Role()
	userID := authenticatedUser.ID()
	if (role == SenderConsumer && userID != conversation.ConsumerID) ||
		(role == SenderProvider && userID != conversation.ProviderID) ||
		(role != SenderConsumer && role != SenderProvider) {
		return nil, ErrConversationAccessDenied
	}

	return authenticatedUser, nil
}

func (s *Service) senderRoleForAuthenticatedParticipant(authID string, foundConversation Conversation) (string, error) {
	workConversation, ok := foundConversation.(*WorkConversation)
	if !ok {
		return "", ErrConversationAccessDenied
	}

	authenticatedUser, err := s.authenticatedUserForConversation(authID, workConversation)
	if err != nil {
		return "", err
	}

	return authenticatedUser.Role(), nil
}

func newParticipantMessage(senderRole, content string, images []filedomain.MessageImage) (*Message, error) {
	if senderRole == SenderConsumer {
		return NewConsumerMessage(content, images...)
	}
	return NewProviderMessage(content, images...)
}

func newParticipantAudioMessage(senderRole string, audio filedomain.MessageAudio) (*Message, error) {
	if senderRole == SenderConsumer {
		return NewConsumerAudioMessage(audio)
	}
	return NewProviderAudioMessage(audio)
}

func newParticipantVideoMessage(senderRole, content string, video filedomain.MessageVideo) (*Message, error) {
	if senderRole == SenderConsumer {
		return NewConsumerVideoMessage(content, video)
	}
	return NewProviderVideoMessage(content, video)
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

func (s *Service) messageAudioForSender(ctx context.Context, authID, fileID string) (*filedomain.MessageAudio, error) {
	preparedAudio, err := s.fileService.PrepareMessageAudio(ctx, authID, fileID)
	if err != nil {
		if errors.Is(err, filedomain.ErrMessageAudioNotAvailable) {
			return nil, ErrMessageAudioNotAvailable
		}
		return nil, fmt.Errorf("preparing message audio: %w", err)
	}
	if preparedAudio == nil {
		return nil, ErrMessageAudioNotAvailable
	}
	return preparedAudio, nil
}

func (s *Service) messageVideoForSender(ctx context.Context, authID, fileID string) (*filedomain.MessageVideo, error) {
	preparedVideo, err := s.fileService.PrepareMessageVideo(ctx, authID, fileID)
	if err != nil {
		if errors.Is(err, filedomain.ErrMessageVideoNotAvailable) {
			return nil, ErrMessageVideoNotAvailable
		}
		return nil, fmt.Errorf("preparing message video: %w", err)
	}
	if preparedVideo == nil {
		return nil, ErrMessageVideoNotAvailable
	}
	return preparedVideo, nil
}

// TODO: Jamás deberíamos devolver fmt.Errorf, siempre errores específicos de dominio
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

func (s *Service) withMessageAttachmentURLs(ctx context.Context, detail *readmodel.ConversationDetail) (*readmodel.ConversationDetail, error) {
	fileIDs := make([]string, 0)
	audioFileIDs := make([]string, 0)
	videoFileIDs := make([]string, 0)
	for _, message := range detail.Messages {
		for _, image := range message.Images {
			fileIDs = append(fileIDs, image.FileID)
		}
		if message.Audio != nil {
			audioFileIDs = append(audioFileIDs, message.Audio.FileID)
		}
		if message.Video != nil {
			videoFileIDs = append(videoFileIDs, message.Video.FileID)
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
	audios, err := s.fileService.ResolveMessageAudios(ctx, audioFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation message audios: %w", err)
	}
	for messageIndex := range detail.Messages {
		audio := detail.Messages[messageIndex].Audio
		if audio == nil {
			continue
		}
		file, ok := audios[audio.FileID]
		if !ok || file.URL == "" {
			return nil, ErrMessageAudioNotAvailable
		}
		*audio = file
	}
	videos, err := s.fileService.ResolveMessageVideos(ctx, videoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation message videos: %w", err)
	}
	for messageIndex := range detail.Messages {
		video := detail.Messages[messageIndex].Video
		if video == nil {
			continue
		}
		file, ok := videos[video.FileID]
		if !ok || file.URL == "" {
			return nil, ErrMessageVideoNotAvailable
		}
		*video = file
	}
	return detail, nil
}

func (s *Service) ensureMessageAllowedInCurrentConversationState(ctx context.Context, foundConversation Conversation, message Message) error {
	if foundConversation.Status() != StatusPending {
		return nil
	}

	if message.SenderRole == SenderProvider {
		return ErrPendingConversationRequiresAcceptance
	}

	sentMessages, err := s.conversationRepository.CountMessagesBySenderRole(ctx, foundConversation.ID(), SenderConsumer)
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
	audioFileIDs := make([]string, 0, len(summaries))
	videoFileIDs := make([]string, 0, len(summaries))
	for i := range summaries {
		if summaries[i].Work != nil {
			fileIDs = append(fileIDs, summaries[i].Work.Counterpart.ProfilePhotoFileID)
		}
		if summaries[i].LastMessage != nil && summaries[i].LastMessage.Audio != nil {
			audioFileIDs = append(audioFileIDs, summaries[i].LastMessage.Audio.FileID)
		}
		if summaries[i].LastMessage != nil && summaries[i].LastMessage.Video != nil {
			videoFileIDs = append(videoFileIDs, summaries[i].LastMessage.Video.FileID)
		}
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

	audiosByFileID, err := s.fileService.ResolveMessageAudios(ctx, audioFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation summary message audios: %w", err)
	}
	for i := range summaries {
		if summaries[i].LastMessage == nil || summaries[i].LastMessage.Audio == nil {
			continue
		}
		audio := summaries[i].LastMessage.Audio
		resolved, ok := audiosByFileID[audio.FileID]
		if !ok || resolved.URL == "" {
			return nil, ErrMessageAudioNotAvailable
		}
		*audio = resolved
	}

	videosByFileID, err := s.fileService.ResolveMessageVideos(ctx, videoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving conversation summary message videos: %w", err)
	}
	for i := range summaries {
		if summaries[i].LastMessage == nil || summaries[i].LastMessage.Video == nil {
			continue
		}
		video := summaries[i].LastMessage.Video
		resolved, ok := videosByFileID[video.FileID]
		if !ok || resolved.URL == "" {
			return nil, ErrMessageVideoNotAvailable
		}
		*video = resolved
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

func (s *Service) assessmentResult(ctx context.Context, chatbotResponse ChatbotResponse, availableCategories []category.Category, coverageZoneID int) (*category.Category, []provider.Provider, map[int]string, *CurrentProviderRecommendation, error) {
	if chatbotResponse.Assessment.Action == ChatbotAssessmentUnchanged {
		return nil, nil, nil, nil, nil
	}

	matchedCategory := categoryForChatbotResponse(chatbotResponse, availableCategories)
	if strings.TrimSpace(chatbotResponse.Assessment.ProblemCategoryName) != "" && matchedCategory == nil {
		return nil, nil, nil, nil, ErrProblemAssessmentInvalid
	}
	if chatbotResponse.Assessment.Outcome == AssessmentProfessionalRequired && matchedCategory == nil {
		return nil, nil, nil, nil, ErrProblemAssessmentInvalid
	}
	if chatbotResponse.Assessment.Outcome != AssessmentProfessionalRequired {
		return matchedCategory, nil, nil, nil, nil
	}

	providers, err := s.userRepository.FindProvidersByCategoryAndCoverageZoneID(ctx, matchedCategory.ID, coverageZoneID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	orderedProviders, reasons, currentRecommendation, err := s.rankEligibleProviders(ctx, providers, chatbotResponse.Assessment)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return matchedCategory, orderedProviders, reasons, currentRecommendation, nil
}

func (s *Service) rankEligibleProviders(ctx context.Context, providers []provider.Provider, assessment ChatbotAssessmentResponse) ([]provider.Provider, map[int]string, *CurrentProviderRecommendation, error) {
	if len(providers) == 0 {
		emptyRecommendation, err := NewCurrentProviderRecommendation(0, nil, nil, s.recommendationConfig.MaxRecommendedProviders)
		return []provider.Provider{}, nil, emptyRecommendation, err
	}

	providerIDs := make([]int, 0, len(providers))
	for _, foundProvider := range providers {
		providerIDs = append(providerIDs, foundProvider.ID())
	}
	evidenceByProviderID, err := s.providerRecommendationEvidence(ctx, providerIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("finding provider recommendation evidence: %w", err)
	}
	candidates := make([]ProviderRecommendationCandidate, 0, len(providers))
	for _, foundProvider := range providers {
		candidates = append(candidates, ProviderRecommendationCandidate{
			Reference:  newProviderRecommendationReference(),
			ProviderID: foundProvider.ID(),
			Evidence:   evidenceByProviderID[foundProvider.ID()],
		})
	}

	ranking, err := s.chatbot.RankProviders(ctx, ProviderRankingRequest{
		ProblemTitle:       assessment.ProblemTitle,
		ProblemDescription: assessment.ProblemDescription,
		MaxResults:         s.recommendationConfig.MaxRecommendedProviders,
		Candidates:         candidates,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if ranking == nil {
		return nil, nil, nil, ErrProviderRecommendationInvalid
	}

	providerByReference := make(map[string]provider.Provider, len(candidates))
	for index, candidate := range candidates {
		providerByReference[candidate.Reference] = providers[index]
	}
	selectedProviders := make([]provider.Provider, 0, len(ranking.Recommendations))
	reasons := make(map[int]string, len(ranking.Recommendations))
	items := make([]ProviderRecommendation, 0, len(ranking.Recommendations))
	seenReferences := make(map[string]struct{}, len(ranking.Recommendations))
	for index, recommendation := range ranking.Recommendations {
		recommendation.Reference = strings.TrimSpace(recommendation.Reference)
		recommendation.Reason = strings.TrimSpace(recommendation.Reason)
		if recommendation.Reference == "" || recommendation.Reason == "" {
			return nil, nil, nil, ErrProviderRecommendationInvalid
		}
		if _, duplicate := seenReferences[recommendation.Reference]; duplicate {
			return nil, nil, nil, ErrProviderRecommendationInvalid
		}
		seenReferences[recommendation.Reference] = struct{}{}
		foundProvider, exists := providerByReference[recommendation.Reference]
		if !exists {
			return nil, nil, nil, ErrProviderRecommendationInvalid
		}
		selectedProviders = append(selectedProviders, foundProvider)
		reasons[foundProvider.ID()] = recommendation.Reason
		items = append(items, ProviderRecommendation{ProviderID: foundProvider.ID(), Position: index + 1, Reason: recommendation.Reason})
	}
	currentRecommendation, err := NewCurrentProviderRecommendation(0, providerIDs, items, s.recommendationConfig.MaxRecommendedProviders)
	if err != nil {
		return nil, nil, nil, err
	}
	summaries, err := s.providersWithProfilePhotoURLs(ctx, selectedProviders)
	if err != nil {
		return nil, nil, nil, err
	}
	return summaries, reasons, currentRecommendation, nil
}

func (s *Service) providerRecommendationEvidence(ctx context.Context, providerIDs []int) (map[int]ProviderRecommendationEvidence, error) {
	ratingStatsByProviderID, err := s.workOrderReader.FindRatingStatsByProviderIDs(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("finding provider rating stats: %w", err)
	}
	workHistoryByProviderID, err := s.workOrderReader.FindPaidWorkHistoryByProviderIDs(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("finding provider paid work history: %w", err)
	}

	evidenceByProviderID := make(map[int]ProviderRecommendationEvidence, len(providerIDs))
	for _, providerID := range providerIDs {
		history := workHistoryByProviderID[providerID]
		if history == nil {
			history = []providerreadmodel.WorkOrder{}
		}
		stats := ratingStatsByProviderID[providerID]
		ratingSummary := stats.Summary()
		mostRecentPaidWork := time.Time{}
		if len(history) > 0 {
			mostRecentPaidWork = history[0].ScheduledOn
		}
		evidenceByProviderID[providerID] = ProviderRecommendationEvidence{
			RatingAverage:      ratingSummary.Average,
			RatingCount:        ratingSummary.Count,
			RatingDistribution: stats.Distribution,
			PaidWorkCount:      len(history),
			MostRecentPaidWork: mostRecentPaidWork,
			WorkHistory:        append([]providerreadmodel.WorkOrder(nil), history...),
		}
	}

	return evidenceByProviderID, nil
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

func (s *Service) withRecommendedProviders(ctx context.Context, detail *readmodel.ConversationDetail, coverageZoneID int) (*readmodel.ConversationDetail, error) {
	if detail.Chatbot == nil || detail.Chatbot.Assessment == nil || detail.Chatbot.Assessment.Outcome != string(AssessmentProfessionalRequired) || detail.Chatbot.Assessment.ProblemCategory == nil {
		return detail, nil
	}

	providers, err := s.userRepository.FindProvidersByCategoryAndCoverageZoneID(ctx, detail.Chatbot.Assessment.ProblemCategory.ID, coverageZoneID)
	if err != nil {
		return nil, err
	}

	detail.Chatbot.RecommendedProviders, err = s.providersWithProfilePhotoURLs(ctx, providers)
	if err != nil {
		return nil, err
	}

	return detail, nil
}

func (s *Service) providersForCurrentRecommendation(ctx context.Context, currentRecommendation *CurrentProviderRecommendation) ([]provider.Provider, error) {
	if currentRecommendation == nil {
		return nil, nil
	}
	providers := make([]provider.Provider, 0, len(currentRecommendation.Recommendations))
	for _, recommendation := range currentRecommendation.Recommendations {
		foundProvider, err := s.userRepository.FindProviderByID(ctx, recommendation.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("finding persisted recommended provider %d: %w", recommendation.ProviderID, err)
		}
		if foundProvider == nil {
			return nil, fmt.Errorf("finding persisted recommended provider %d: %w", recommendation.ProviderID, ErrProviderRecommendationInvalid)
		}
		providers = append(providers, *foundProvider)
	}
	return s.providersWithProfilePhotoURLs(ctx, providers)
}

func (s *Service) providersWithProfilePhotoURLs(ctx context.Context, providers []provider.Provider) ([]provider.Provider, error) {
	profilePhotoFileIDs := make([]string, 0, len(providers))
	for i := range providers {
		if providers[i].ProfilePhoto() == nil {
			continue
		}
		profilePhotoFileIDs = append(profilePhotoFileIDs, providers[i].ProfilePhoto().FileID)
	}

	profilePhotoURLs, err := s.fileService.ResolvePublicURLs(ctx, profilePhotoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving chatbot recommended provider profile photo urls: %w", err)
	}

	return provider.WithProfilePhotoURLs(providers, profilePhotoURLs), nil
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
