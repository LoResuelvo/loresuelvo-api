package conversation_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type conversationRepositoryMock struct {
	savedConversation      conversation.Conversation
	savedMessage           conversation.Message
	addedMessage           conversation.Message
	savedResult            conversation.Conversation
	addedResult            *conversation.Message
	savedChatbot           conversation.Conversation
	savedChatbotResult     conversation.Conversation
	updatedConversation    conversation.Conversation
	existsValue            bool
	existsCalled           bool
	saveCalled             bool
	updateCalled           bool
	updateCalls            int
	addMessageCalled       bool
	findByIDCalled         bool
	countCalled            bool
	startProcessingCalled  bool
	finishProcessingCalled bool
	startProcessingErr     error
	foundResult            conversation.Conversation
	countResult            int
	err                    error
}

func (r *conversationRepositoryMock) ExistsBetween(consumerID, providerID int) (bool, error) {
	r.existsCalled = true
	if r.err != nil {
		return false, r.err
	}
	return r.existsValue, nil
}

func (r *conversationRepositoryMock) SaveConversation(ctx context.Context, c conversation.Conversation) (conversation.Conversation, error) {
	r.saveCalled = true
	r.savedConversation = c
	if c.ConversationType() == conversation.TypeChatbot {
		r.savedChatbot = c
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.savedResult != nil {
		return r.savedResult, nil
	}
	if r.savedChatbotResult != nil {
		return r.savedChatbotResult, nil
	}
	c.Base().ID = 1
	messages := c.Messages()
	for index := range messages {
		messages[index].ID = index + 1
		messages[index].ConversationID = c.Base().ID
		messages[index].CreatedOn = time.Now()
	}
	c.SetMessages(messages)
	if len(messages) > 0 {
		r.savedMessage = messages[0]
	}
	return c, nil
}

func (r *conversationRepositoryMock) FindByID(ctx context.Context, conversationID int) (conversation.Conversation, error) {
	r.findByIDCalled = true
	if r.err != nil {
		return nil, r.err
	}
	if r.foundResult != nil {
		return r.foundResult, nil
	}
	return nil, conversation.ErrConversationDoesNotExist
}

func (r *conversationRepositoryMock) AddMessage(ctx context.Context, conversationID int, m conversation.Message) (*conversation.Message, error) {
	r.addMessageCalled = true
	r.addedMessage = m
	if r.err != nil {
		return nil, r.err
	}
	if r.addedResult != nil {
		return r.addedResult, nil
	}
	m.ID = 2
	m.ConversationID = conversationID
	m.CreatedOn = time.Now()
	return &m, nil
}

func (r *conversationRepositoryMock) UpdateConversation(ctx context.Context, c conversation.Conversation) (conversation.Conversation, error) {
	r.updateCalled = true
	r.updateCalls++
	r.updatedConversation = c

	if chatbotConversation, ok := c.(*conversation.ChatBotConversation); ok {
		if chatbotConversation.ProcessingStartedAt() != nil {
			r.startProcessingCalled = true
			if r.startProcessingErr != nil {
				return nil, r.startProcessingErr
			}
		} else if r.startProcessingCalled {
			r.finishProcessingCalled = true
		}
	}

	if r.err != nil {
		return nil, r.err
	}

	messages := c.Messages()
	for index := range messages {
		if messages[index].ID == 0 {
			messages[index].ID = index + 1
			messages[index].ConversationID = c.Base().ID
			messages[index].CreatedOn = time.Now()
		}
	}
	c.SetMessages(messages)

	return c, nil
}

func (r *conversationRepositoryMock) CountMessagesBySenderRole(ctx context.Context, conversationID int, senderRole string) (int, error) {
	r.countCalled = true
	if r.err != nil {
		return 0, r.err
	}
	return r.countResult, nil
}

type consumerIDFinderMock struct {
	consumerID int
	err        error
}

func (m *consumerIDFinderMock) FindIDByAuthID(authID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.consumerID, nil
}

func (m *consumerIDFinderMock) FindAuthIDByID(id int) (string, error) {
	return "", nil
}

type providerIDFinderMock struct {
	providerID             int
	err                    error
	providers              []provider.Provider
	requestedCategoryID    int
	findByCategoryIDCalled bool
	findByCategoryIDErr    error
}

func (m *providerIDFinderMock) FindIDByAuthID(authID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.providerID, nil
}

func (m *providerIDFinderMock) FindAuthIDByID(id int) (string, error) {
	return "", nil
}

func (m *providerIDFinderMock) FindByCategoryID(categoryID int) ([]provider.Provider, error) {
	m.findByCategoryIDCalled = true
	m.requestedCategoryID = categoryID
	if m.findByCategoryIDErr != nil {
		return nil, m.findByCategoryIDErr
	}
	return m.providers, nil
}

type recommendationCategoryListerMock struct {
	categories []category.Category
	called     bool
	err        error
}

func (m *recommendationCategoryListerMock) ListAll() ([]category.Category, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return m.categories, nil
}

type conversationReaderMock struct {
	consumerSummaries         []readmodel.ConversationSummary
	providerSummaries         []readmodel.ConversationSummary
	chatbotSummaries          []readmodel.ConversationSummary
	detail                    *readmodel.ConversationDetail
	requestedParticipantID    int
	requestedParticipantRole  string
	requestedConversationType string
	requestedConversationID   int
	err                       error
}

func (m *conversationReaderMock) FindSummariesByParticipantIDRoleAndType(ctx context.Context, participantID int, participantRole string, conversationType string) ([]readmodel.ConversationSummary, error) {
	m.requestedParticipantID = participantID
	m.requestedParticipantRole = participantRole
	m.requestedConversationType = conversationType
	if m.err != nil {
		return nil, m.err
	}
	if conversationType == conversation.TypeChatbot {
		return m.chatbotSummaries, nil
	}
	if participantRole == conversation.SenderProvider {
		return m.providerSummaries, nil
	}
	return m.consumerSummaries, nil
}

func (m *conversationReaderMock) FindDetailByIDRoleAndType(ctx context.Context, conversationID int, participantRole string, conversationType string) (*readmodel.ConversationDetail, error) {
	m.requestedConversationID = conversationID
	m.requestedParticipantRole = participantRole
	m.requestedConversationType = conversationType
	if m.err != nil {
		return nil, m.err
	}
	return m.detail, nil
}

type fileURLResolverMock struct {
	resolvedFileIDs []string
	urlsByFileID    map[string]string
	err             error
}

func (m *fileURLResolverMock) PrepareMessageImages(_ context.Context, _ string, fileIDs []string) ([]filedomain.MessageImage, error) {
	if len(fileIDs) == 0 {
		return []filedomain.MessageImage{}, nil
	}
	files := make([]filedomain.MessageImage, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		files = append(files, filedomain.MessageImage{FileID: fileID, OriginalName: fileID + ".jpg", URL: "https://files/" + fileID})
	}
	return files, m.err
}

func (m *fileURLResolverMock) PrepareChatbotMessageImages(_ context.Context, _ string, fileIDs []string) ([]filedomain.MessageImageContent, error) {
	if len(fileIDs) == 0 {
		return []filedomain.MessageImageContent{}, nil
	}
	files := make([]filedomain.MessageImageContent, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		files = append(files, filedomain.MessageImageContent{
			MessageImage: filedomain.MessageImage{FileID: fileID, OriginalName: fileID + ".jpg", URL: "https://files/" + fileID},
			MimeType:     "image/jpeg",
			Data:         []byte("image-bytes-" + fileID),
		})
	}
	return files, m.err
}

func (m *fileURLResolverMock) ResolveMessageImages(_ context.Context, fileIDs []string) (map[string]filedomain.MessageImage, error) {
	if len(fileIDs) == 0 {
		return map[string]filedomain.MessageImage{}, nil
	}
	if m.err != nil {
		return nil, m.err
	}
	files := make(map[string]filedomain.MessageImage, len(fileIDs))
	for _, fileID := range fileIDs {
		files[fileID] = filedomain.MessageImage{FileID: fileID, OriginalName: fileID + ".jpg", URL: "https://files/" + fileID}
	}
	return files, nil
}

func (m *fileURLResolverMock) ResolvePublicURL(ctx context.Context, fileID string) (string, error) {
	m.resolvedFileIDs = []string{fileID}
	urlsByFileID, err := m.ResolvePublicURLs(ctx, []string{fileID})
	if err != nil {
		return "", err
	}

	return urlsByFileID[fileID], nil
}

func (m *fileURLResolverMock) ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error) {
	m.resolvedFileIDs = fileIDs
	if m.err != nil {
		return nil, m.err
	}
	if m.urlsByFileID != nil {
		return m.urlsByFileID, nil
	}
	return map[string]string{}, nil
}

func categoryNames(categories []category.Category) []string {
	names := make([]string, 0, len(categories))
	for _, category := range categories {
		names = append(names, category.Name)
	}

	return names
}

type messagePublisherMock struct {
	publishedConversation conversation.Conversation
	publishedSenderAuthID string
	publishedMessage      conversation.Message
	publishCalled         bool
}

func (m *messagePublisherMock) PublishMessage(ctx context.Context, conv conversation.Conversation, senderAuthID string, msg conversation.Message) {
	m.publishCalled = true
	m.publishedConversation = conv
	m.publishedSenderAuthID = senderAuthID
	m.publishedMessage = msg
}

type chatbotMock struct {
	response            *conversation.ChatbotResponse
	called              bool
	question            conversation.ChatbotHomeProblemQuestion
	availableCategories []category.Category
	summary             string
	summaryCalled       bool
	previousSummary     string
	summaryMessages     []conversation.Message
	err                 error
}

func (m *chatbotMock) AnswerHomeProblemQuestion(ctx context.Context, question conversation.ChatbotHomeProblemQuestion, availableCategories []category.Category) (*conversation.ChatbotResponse, error) {
	m.called = true
	m.question = question
	m.availableCategories = availableCategories
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil && m.response.Assessment.Action == "" {
		m.response.Assessment = conversation.ChatbotAssessmentResponse{
			Action:  conversation.ChatbotAssessmentReplace,
			Outcome: conversation.AssessmentCollectingInformation,
		}
	}
	if m.response != nil && len(question.Images) > 0 && len(m.response.ImageDescriptions) == 0 {
		for _, image := range question.Images {
			m.response.ImageDescriptions = append(m.response.ImageDescriptions, conversation.ChatbotImageDescription{
				ImageRef: conversation.ChatbotImageRef(image.FileID), Description: "Descripción de prueba",
			})
		}
	}
	return m.response, nil
}

func (m *chatbotMock) SummarizeHomeProblemConversation(ctx context.Context, previousSummary string, messages []conversation.Message) (string, error) {
	m.summaryCalled = true
	m.previousSummary = previousSummary
	m.summaryMessages = messages
	if m.err != nil {
		return "", m.err
	}
	if strings.TrimSpace(m.summary) != "" {
		return m.summary, nil
	}

	return "Resumen actualizado", nil
}

func TestCreateChatbotConversationCreatesActiveConversationWithConsumerAndChatbotMessages(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Pérdida de agua en la cocina",
		Content: "Cerrá la llave de paso y revisá el sifón.",
	}}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, publisher, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "  Tengo una pérdida de agua en la cocina  ")

	require.NoError(t, err)
	require.NotNil(t, createdResult)
	assert.True(t, chatbot.called)
	assert.Equal(t, "Tengo una pérdida de agua en la cocina", chatbot.question.UserMessage)
	assert.False(t, chatbot.summaryCalled)
	assert.Empty(t, chatbot.summaryMessages)
	assert.True(t, repo.saveCalled)
	savedChatbotConversation := repo.savedChatbot.(*conversation.ChatBotConversation)
	assert.Equal(t, conversation.TypeChatbot, savedChatbotConversation.ConversationType())
	assert.Equal(t, 10, savedChatbotConversation.ConsumerID)
	assert.Equal(t, "Pérdida de agua en la cocina", savedChatbotConversation.Title)
	assert.Empty(t, savedChatbotConversation.Context.Summary)
	assert.Zero(t, savedChatbotConversation.Context.LastSummarizedMessageID)
	assert.Equal(t, conversation.ChatbotResponseAnswered, createdResult.ResponseStatus)
	assert.Equal(t, conversation.StatusActive, savedChatbotConversation.Base().Status)
	require.Len(t, savedChatbotConversation.Messages(), 2)
	assert.Equal(t, conversation.SenderConsumer, savedChatbotConversation.Messages()[0].SenderRole)
	assert.Equal(t, "Tengo una pérdida de agua en la cocina", savedChatbotConversation.Messages()[0].Content)
	assert.Equal(t, conversation.SenderChatbot, savedChatbotConversation.Messages()[1].SenderRole)
	assert.Equal(t, "Cerrá la llave de paso y revisá el sifón.", savedChatbotConversation.Messages()[1].Content)
	assert.False(t, publisher.publishCalled)
}

func TestCreateChatbotConversationAcceptsImageOnlyMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Humedad en pared",
		Content: "La imagen muestra humedad compatible con una filtración.",
	}}
	fileService := &fileURLResolverMock{}

	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, fileService, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "   ", []string{"image-file-id"})

	require.NoError(t, err)
	require.NotNil(t, createdResult)
	assert.True(t, chatbot.called)
	assert.Empty(t, chatbot.question.UserMessage)
	require.Len(t, chatbot.question.Images, 1)
	assert.Equal(t, "image-file-id", chatbot.question.Images[0].FileID)
	savedChatbotConversation := repo.savedChatbot.(*conversation.ChatBotConversation)
	require.Len(t, savedChatbotConversation.Messages(), 2)
	assert.Empty(t, savedChatbotConversation.Messages()[0].Content)
	assert.Equal(t, []filedomain.MessageImage{{FileID: "image-file-id", OriginalName: "image-file-id.jpg", URL: "https://files/image-file-id", Description: "Descripción de prueba"}}, savedChatbotConversation.Messages()[0].Images)
}

func TestCreateChatbotConversationIncludesRecommendedProvidersWhenDiagnosisIsCompleted(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	plumbingCategory := &category.Category{ID: 3, Name: "Plomería", NormalizedName: "plomería"}
	recommendedProvider, err := provider.NewProvider("auth0|provider", "juan@example.com", "Juan", "Gómez", plumbingCategory, "provider-photo-file-id")
	require.NoError(t, err)
	recommendedProvider.ID = 20
	categoryLister := &recommendationCategoryListerMock{categories: []category.Category{*plumbingCategory}}
	providerFinder := &providerIDFinderMock{providers: []provider.Provider{*recommendedProvider}}
	fileURLResolver := &fileURLResolverMock{urlsByFileID: map[string]string{"provider-photo-file-id": "https://cdn/provider.jpg"}}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Pérdida de agua en la cocina",
		Content: "Contactá a un plomero.",
		Assessment: conversation.ChatbotAssessmentResponse{
			Action: conversation.ChatbotAssessmentReplace, Outcome: conversation.AssessmentProfessionalRequired,
			ProblemTitle: "Pérdida de agua", ProblemDescription: "Pierde agua la bacha.", ProblemCategoryName: "Plomería",
		},
	}}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, categoryLister, fileURLResolver, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Pierde agua la bacha")

	require.NoError(t, err)
	require.NotNil(t, createdResult)
	require.NotNil(t, createdResult.Assessment)
	assert.Equal(t, conversation.AssessmentProfessionalRequired, createdResult.Assessment.Outcome)
	require.NotNil(t, createdResult.ProblemCategory)
	assert.Equal(t, 3, createdResult.ProblemCategory.ID)
	assert.Equal(t, "Plomería", createdResult.ProblemCategory.Name)
	assert.True(t, categoryLister.called)
	assert.Equal(t, []string{"Plomería"}, categoryNames(chatbot.availableCategories))
	assert.True(t, providerFinder.findByCategoryIDCalled)
	assert.Equal(t, 3, providerFinder.requestedCategoryID)
	assert.Equal(t, []string{"provider-photo-file-id"}, fileURLResolver.resolvedFileIDs)
	require.Len(t, createdResult.RecommendedProviders, 1)
	assert.Equal(t, 20, createdResult.RecommendedProviders[0].ID)
	assert.Equal(t, "Juan", createdResult.RecommendedProviders[0].Name)
	assert.Equal(t, "Gómez", createdResult.RecommendedProviders[0].Surname)
	assert.Equal(t, "Plomería", createdResult.RecommendedProviders[0].CategoryName)
	assert.Equal(t, "https://cdn/provider.jpg", createdResult.RecommendedProviders[0].ProfilePhotoURL)
}

func TestCreateChatbotConversationDoesNotRecommendProvidersBeforeDiagnosisIsCompleted(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	categoryLister := &recommendationCategoryListerMock{categories: []category.Category{{ID: 3, Name: "Plomería", NormalizedName: "plomería"}}}
	providerFinder := &providerIDFinderMock{}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Consulta de humedad",
		Content: "Necesito más información.",
		Assessment: conversation.ChatbotAssessmentResponse{
			Action: conversation.ChatbotAssessmentReplace, Outcome: conversation.AssessmentCollectingInformation,
		},
	}}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, categoryLister, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Tengo humedad")

	require.NoError(t, err)
	require.NotNil(t, createdResult)
	require.NotNil(t, createdResult.Assessment)
	assert.Equal(t, conversation.AssessmentCollectingInformation, createdResult.Assessment.Outcome)
	assert.Nil(t, createdResult.ProblemCategory)
	assert.Empty(t, createdResult.RecommendedProviders)
	assert.False(t, providerFinder.findByCategoryIDCalled)
	assert.True(t, categoryLister.called)
	assert.Equal(t, []string{"Plomería"}, categoryNames(chatbot.availableCategories))
}

func TestCreateChatbotConversationRejectsNonConsumerUser(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Status: conversation.ChatbotResponseAnswered, Title: "Consulta", Content: "Respuesta"}}

	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|provider", "Tengo una pérdida de agua")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanMessageChatbot)
	assert.Nil(t, createdResult)
	assert.False(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestCreateChatbotConversationRejectsEmptyMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Status: conversation.ChatbotResponseAnswered, Title: "Consulta", Content: "Respuesta"}}

	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdResult)
	assert.False(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestCreateChatbotConversationSendsQuestionToChatbotWithoutKeywordPreFilter(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Status: conversation.ChatbotResponseOutOfScope, Title: "Consulta fuera de alcance", Content: "Solo puedo responder consultas sobre problemas del hogar."}}

	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "¿Qué equipo ganó el último partido de fútbol?")

	require.NoError(t, err)
	require.NotNil(t, createdResult)
	assert.True(t, chatbot.called)
	assert.Equal(t, "¿Qué equipo ganó el último partido de fútbol?", chatbot.question.UserMessage)
	assert.Equal(t, conversation.ChatbotResponseOutOfScope, createdResult.ResponseStatus)
	assert.True(t, repo.saveCalled)
}

func TestCreateChatbotConversationReturnsChatbotErrors(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{err: errors.New("chatbot unavailable")}

	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Tengo una pérdida de agua")

	assert.Error(t, err)
	assert.Nil(t, createdResult)
	assert.True(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestContinueChatbotConversationAddsConsumerAndChatbotMessagesToExistingChatbotConversation(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: &conversation.ChatBotConversation{
			BaseConversation: &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
			ConsumerID:       10,
			Title:            "Pérdida de agua en la cocina",
			Context: conversation.ChatbotConversationContext{
				Summary:                 "La consumidora tiene una pérdida debajo de la pileta.",
				LastSummarizedMessageID: 1,
			},
		},
	}
	repo.foundResult.AddMessage(conversation.Message{ID: 1, ConversationID: 7, SenderRole: conversation.SenderConsumer, Content: "El agua aparece cuando uso la bacha.", CreatedOn: time.Now()})
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Pérdida de agua en la cocina",
		Content: "Revisá el sifón.",
	}}
	publisher := &messagePublisherMock{}
	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, publisher, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|consumer", 7, "  Parece salir de la rosca del sifón.  ")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, repo.startProcessingCalled)
	assert.True(t, repo.updateCalled)
	assert.True(t, repo.finishProcessingCalled)
	assert.Equal(t, "La consumidora tiene una pérdida debajo de la pileta.", chatbot.question.ContextSummary)
	require.Len(t, chatbot.question.RecentMessages, 1)
	assert.Equal(t, "El agua aparece cuando uso la bacha.", chatbot.question.RecentMessages[0].Content)
	assert.Equal(t, "Parece salir de la rosca del sifón.", chatbot.question.UserMessage)
	assert.False(t, chatbot.summaryCalled)
	assert.Empty(t, chatbot.summaryMessages)
	require.Len(t, result.Conversation.Messages(), 3)
	assert.Equal(t, conversation.SenderConsumer, result.Conversation.Messages()[1].SenderRole)
	assert.Equal(t, "Parece salir de la rosca del sifón.", result.Conversation.Messages()[1].Content)
	assert.Equal(t, conversation.SenderChatbot, result.Conversation.Messages()[2].SenderRole)
	assert.Equal(t, "Revisá el sifón.", result.Conversation.Messages()[2].Content)
	assert.False(t, publisher.publishCalled)
}

func TestContinueChatbotConversationSendsCurrentTurnImagesToChatbot(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: &conversation.ChatBotConversation{
			BaseConversation: &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
			ConsumerID:       10,
			Title:            "Pérdida de agua en la cocina",
		},
	}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Pérdida de agua en la cocina",
		Content: "Por la imagen, revisá la rosca del sifón.",
	}}
	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|consumer", 7, "Saqué una foto", []string{"detail-image-id"})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, chatbot.question.Images, 1)
	assert.Equal(t, "detail-image-id", chatbot.question.Images[0].FileID)
	require.Len(t, result.Conversation.Messages(), 2)
	assert.Equal(t, []filedomain.MessageImage{{FileID: "detail-image-id", OriginalName: "detail-image-id.jpg", URL: "https://files/detail-image-id", Description: "Descripción de prueba"}}, result.Conversation.Messages()[0].Images)
}

func TestContinueChatbotConversationSummarizesPendingContextWhenRecentMessageLimitIsReached(t *testing.T) {
	chatbotConversation := &conversation.ChatBotConversation{
		BaseConversation: &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
		ConsumerID:       10,
		Title:            "Pérdida de agua en la cocina",
		Context: conversation.ChatbotConversationContext{
			Summary: "Resumen previo",
		},
	}
	for messageID := 1; messageID <= conversation.ChatbotRecentMessageLimit; messageID++ {
		chatbotConversation.AddMessage(conversation.Message{
			ID:             messageID,
			ConversationID: 7,
			SenderRole:     conversation.SenderConsumer,
			Content:        fmt.Sprintf("Mensaje pendiente %d", messageID),
			CreatedOn:      time.Now(),
		})
	}
	repo := &conversationRepositoryMock{foundResult: chatbotConversation}
	chatbot := &chatbotMock{
		response: &conversation.ChatbotResponse{
			Status:  conversation.ChatbotResponseAnswered,
			Title:   "Pérdida de agua en la cocina",
			Content: "Seguimos revisando el problema.",
		},
		summary: "Resumen compactado",
	}
	service := conversation.NewService(repo, &consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|consumer", 7, "Nueva información")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, chatbot.summaryCalled)
	assert.Equal(t, "Resumen previo", chatbot.previousSummary)
	require.Len(t, chatbot.summaryMessages, conversation.ChatbotRecentMessageLimit)
	assert.Equal(t, 1, chatbot.summaryMessages[0].ID)
	assert.Equal(t, conversation.ChatbotRecentMessageLimit, chatbot.summaryMessages[conversation.ChatbotRecentMessageLimit-1].ID)
	assert.Equal(t, "Resumen compactado", chatbot.question.ContextSummary)
	assert.Equal(t, "Nueva información", chatbot.question.UserMessage)
	assert.Equal(t, 3, repo.updateCalls)
	foundChatbotConversation := result.Conversation.(*conversation.ChatBotConversation)
	assert.Equal(t, "Resumen compactado", foundChatbotConversation.Context.Summary)
	assert.Equal(t, conversation.ChatbotRecentMessageLimit, foundChatbotConversation.Context.LastSummarizedMessageID)
	require.Len(t, foundChatbotConversation.Messages(), conversation.ChatbotRecentMessageLimit+2)
}

func TestContinueChatbotConversationRejectsAnotherConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: &conversation.ChatBotConversation{
		BaseConversation: &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
		ConsumerID:       10,
		Title:            "Pérdida de agua",
	}}
	service := conversation.NewService(repo, &consumerIDFinderMock{consumerID: 99}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|other", 7, "Hola")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, result)
	assert.False(t, repo.startProcessingCalled)
	assert.False(t, repo.updateCalled)
}

func TestContinueChatbotConversationReturnsProcessingConflict(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: &conversation.ChatBotConversation{
			BaseConversation: &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
			ConsumerID:       10,
			Title:            "Pérdida de agua",
		},
		startProcessingErr: conversation.ErrChatbotConversationAlreadyProcessing,
	}
	chatbot := &chatbotMock{}
	now := time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC)
	service := conversation.NewService(repo, &consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: now})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|consumer", 7, "Hola")

	assert.ErrorIs(t, err, conversation.ErrChatbotConversationAlreadyProcessing)
	assert.Nil(t, result)
	assert.True(t, repo.startProcessingCalled)
	assert.False(t, repo.finishProcessingCalled)
	assert.False(t, chatbot.called)
	updatedChatbotConversation := repo.updatedConversation.(*conversation.ChatBotConversation)
	require.NotNil(t, updatedChatbotConversation.ProcessingStartedAt())
	assert.Equal(t, now, *updatedChatbotConversation.ProcessingStartedAt())
}

func TestGetByIDReturnsConversationDetailForParticipantConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	conversationReader := &conversationReaderMock{detail: conversationDetailFixture(conversation.SenderProvider)}
	fileURLResolver := &fileURLResolverMock{urlsByFileID: map[string]string{"provider-photo-file-id": "https://cdn/provider.jpg"}}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 1, foundConversation.ID)
	require.NotNil(t, foundConversation.Work)
	assert.Equal(t, 20, foundConversation.Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, foundConversation.Work.Counterpart.Role)
	assert.Equal(t, "https://cdn/provider.jpg", foundConversation.Work.Counterpart.ProfilePhotoURL)
	assert.Equal(t, []string{"provider-photo-file-id"}, fileURLResolver.resolvedFileIDs)
	assert.Len(t, foundConversation.Messages, 1)
}

func TestGetByIDReturnsConversationDetailForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	conversationReader := &conversationReaderMock{detail: conversationDetailFixture(conversation.SenderConsumer)}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, conversationReader, nil, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|provider", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	require.NotNil(t, foundConversation.Work)
	assert.Equal(t, 10, foundConversation.Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, foundConversation.Work.Counterpart.Role)
}

func TestGetByIDReturnsChatbotConversationDetailForOwnerConsumer(t *testing.T) {
	recommendedCategoryID := 3
	repo := &conversationRepositoryMock{foundResult: &conversation.ChatBotConversation{
		BaseConversation:   &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
		ConsumerID:         10,
		Title:              "Pérdida de agua en la cocina",
		LastResponseStatus: conversation.ChatbotResponseAnswered,
		CurrentAssessment: &conversation.ProblemAssessment{
			ID: 1, Version: 1, Outcome: conversation.AssessmentProfessionalRequired,
			ProblemCategoryID: &recommendedCategoryID, ProblemTitle: "Pérdida", ProblemDescription: "Pierde agua", BasedOnMessageID: 1,
		},
	}}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	plumbingCategory := &category.Category{ID: recommendedCategoryID, Name: "Plomería", NormalizedName: "plomería"}
	recommendedProvider, err := provider.NewProvider("auth0|provider", "juan@example.com", "Juan", "Gómez", plumbingCategory, "provider-photo-file-id")
	require.NoError(t, err)
	recommendedProvider.ID = 20
	providerFinder := &providerIDFinderMock{providers: []provider.Provider{*recommendedProvider}}
	conversationReader := &conversationReaderMock{detail: &readmodel.ConversationDetail{
		ID:     7,
		Type:   conversation.TypeChatbot,
		Status: conversation.StatusActive,
		Chatbot: &readmodel.ChatbotConversationDetail{
			Title:          "Pérdida de agua en la cocina",
			ResponseStatus: string(conversation.ChatbotResponseAnswered),
			Assessment: &readmodel.ProblemAssessmentDetail{
				Outcome:         string(conversation.AssessmentProfessionalRequired),
				ProblemCategory: &readmodel.ProblemCategory{ID: recommendedCategoryID, Name: "Plomería"},
			},
		},
		Messages: []readmodel.MessageDetail{{ID: 1, SenderRole: conversation.SenderConsumer, Content: "Pierde agua"}},
	}}
	fileURLResolver := &fileURLResolverMock{urlsByFileID: map[string]string{"provider-photo-file-id": "https://cdn/provider.jpg"}}
	service := conversation.NewService(repo, consumerIDFinder, providerFinder, conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 7)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 7, conversationReader.requestedConversationID)
	assert.Equal(t, conversation.SenderConsumer, conversationReader.requestedParticipantRole)
	assert.Equal(t, conversation.TypeChatbot, conversationReader.requestedConversationType)
	assert.True(t, providerFinder.findByCategoryIDCalled)
	assert.Equal(t, recommendedCategoryID, providerFinder.requestedCategoryID)
	require.NotNil(t, foundConversation.Chatbot)
	require.Len(t, foundConversation.Chatbot.RecommendedProviders, 1)
	assert.Equal(t, 20, foundConversation.Chatbot.RecommendedProviders[0].ID)
	assert.Equal(t, "https://cdn/provider.jpg", foundConversation.Chatbot.RecommendedProviders[0].ProfilePhotoURL)
}

func TestGetByIDRejectsChatbotConversationForNonOwnerConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: &conversation.ChatBotConversation{
		BaseConversation: &conversation.BaseConversation{ID: 7, Type: conversation.TypeChatbot, Status: conversation.StatusActive},
		ConsumerID:       10,
	}}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	conversationReader := &conversationReaderMock{}
	service := conversation.NewService(repo, consumerIDFinder, &providerIDFinderMock{}, conversationReader, nil, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|other-consumer", 7)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Zero(t, conversationReader.requestedConversationID)
}

func TestGetByIDRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerIDFinder := &providerIDFinderMock{providerID: 888}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|other", 1)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestGetByIDReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestSendMessageAddsConsumerMessageForParticipantConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "  ¿El jueves te queda cómodo?  ", nil)

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.True(t, repo.addMessageCalled)
	assert.Equal(t, conversation.SenderConsumer, repo.addedMessage.SenderRole)
	assert.Equal(t, "¿El jueves te queda cómodo?", repo.addedMessage.Content)
	assert.Equal(t, 1, sentMessage.ConversationID)
}

func TestSendMessageAddsResolvedPrivateImages(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	publisher := &messagePublisherMock{}
	fileService := &fileURLResolverMock{}
	service := conversation.NewService(repo, &consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}, &conversationReaderMock{}, publisher, &chatbotMock{}, nil, fileService, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "", []string{"image-file-id"})

	require.NoError(t, err)
	require.Len(t, sentMessage.Images, 1)
	assert.Equal(t, "image-file-id", sentMessage.Images[0].FileID)
	assert.Equal(t, "image-file-id.jpg", sentMessage.Images[0].OriginalName)
	assert.Equal(t, "https://files/image-file-id", sentMessage.Images[0].URL)
	assert.Equal(t, sentMessage.Images, publisher.publishedMessage.Images)
}

func TestSendMessageRejectsUnavailableImageBeforePersistence(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	fileService := &fileURLResolverMock{err: filedomain.ErrMessageImageNotAvailable}
	service := conversation.NewService(repo, &consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, fileService, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Problem", []string{"image-file-id"})

	assert.Nil(t, sentMessage)
	assert.ErrorIs(t, err, conversation.ErrMessageImageNotAvailable)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageAddsProviderMessageForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "Sí, puedo pasar el jueves a las 10", nil)

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.True(t, repo.addMessageCalled)
	assert.Equal(t, conversation.SenderProvider, repo.addedMessage.SenderRole)
	assert.Equal(t, "Sí, puedo pasar el jueves a las 10", repo.addedMessage.Content)
	assert.Equal(t, 1, sentMessage.ConversationID)
}

func TestSendMessageRejectsProviderMessageInPendingConversation(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "Sí, puedo pasar el jueves a las 10", nil)

	assert.ErrorIs(t, err, conversation.ErrPendingConversationRequiresAcceptance)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageRejectsConsumerMessageWhenPendingLimitWasReached(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: conversationFixture(),
		countResult: conversation.PendingConsumerMessageLimit,
	}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Otro detalle", nil)

	assert.ErrorIs(t, err, conversation.ErrPendingConversationMessageLimitReached)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.countCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerIDFinder := &providerIDFinderMock{providerID: 888}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|other", 1, "Hola", nil)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 999, "Hola", nil)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageRejectsEmptyContent(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "   ", nil)

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
	assert.False(t, publisher.publishCalled)
}

func TestSendMessagePublishesMessageAfterSuccessfulPersistence(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Hola proveedor", nil)

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, publisher.publishCalled)
	assert.Equal(t, 1, publisher.publishedConversation.Base().ID)
	assert.Equal(t, "auth0|consumer", publisher.publishedSenderAuthID)
}

func TestSendMessageDoesNotPublishWhenPersistFails(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture(), err: errors.New("database error")}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Hola", nil)

	assert.Error(t, err)
	assert.Nil(t, sentMessage)
	assert.False(t, publisher.publishCalled)
}

func TestListReturnsConversationSummariesForConsumer(t *testing.T) {
	now := time.Now()
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	conversationReader := &conversationReaderMock{
		consumerSummaries: []readmodel.ConversationSummary{
			{
				ID:     1,
				Type:   conversation.TypeWork,
				Status: conversation.StatusPending,
				Work: &readmodel.WorkConversationSummary{Counterpart: readmodel.ConversationParticipant{
					ID:                 20,
					Role:               conversation.SenderProvider,
					Name:               "Juan",
					Surname:            "Gómez",
					CategoryName:       "Plomería",
					ProfilePhotoFileID: "provider-photo-file-id",
				}},
				LastMessage: &readmodel.MessageSummary{ID: 1, SenderRole: conversation.SenderConsumer, Content: "Hola", CreatedOn: now},
				UpdatedOn:   now,
			},
		},
	}

	fileURLResolver := &fileURLResolverMock{urlsByFileID: map[string]string{"provider-photo-file-id": "https://cdn/provider.jpg"}}
	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListWorkConversations(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, 1, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, 20, summaries[0].Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, summaries[0].Work.Counterpart.Role)
	assert.Equal(t, "Juan", summaries[0].Work.Counterpart.Name)
	assert.Equal(t, "Gómez", summaries[0].Work.Counterpart.Surname)
	assert.Equal(t, "Plomería", summaries[0].Work.Counterpart.CategoryName)
	assert.Equal(t, "https://cdn/provider.jpg", summaries[0].Work.Counterpart.ProfilePhotoURL)
	assert.Equal(t, []string{"provider-photo-file-id"}, fileURLResolver.resolvedFileIDs)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, "Hola", summaries[0].LastMessage.Content)
}

func TestListReturnsConversationSummariesForProvider(t *testing.T) {
	now := time.Now()
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	conversationReader := &conversationReaderMock{
		providerSummaries: []readmodel.ConversationSummary{
			{
				ID:     1,
				Type:   conversation.TypeWork,
				Status: conversation.StatusPending,
				Work: &readmodel.WorkConversationSummary{Counterpart: readmodel.ConversationParticipant{
					ID:      10,
					Role:    conversation.SenderConsumer,
					Name:    "Ana",
					Surname: "Pérez",
				}},
				LastMessage: &readmodel.MessageSummary{ID: 1, SenderRole: conversation.SenderConsumer, Content: "Hola", CreatedOn: now},
				UpdatedOn:   now,
			},
		},
	}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, conversationReader, nil, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListWorkConversations(context.Background(), "auth0|provider")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, 1, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, 10, summaries[0].Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].Work.Counterpart.Role)
	assert.Equal(t, "Ana", summaries[0].Work.Counterpart.Name)
	assert.Equal(t, "Pérez", summaries[0].Work.Counterpart.Surname)
	assert.Empty(t, summaries[0].Work.Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
}

func TestListRejectsAuthenticatedUserWithoutParticipantProfile(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := conversation.NewService(repo, consumerIDFinder, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListWorkConversations(context.Background(), "auth0|unknown")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, summaries)
}

func conversationDetailFixture(counterpartRole string) *readmodel.ConversationDetail {
	now := time.Now()
	counterpartID := 20
	counterpartName := "Juan"
	counterpartSurname := "Gómez"
	counterpartCategory := "Plomería"
	profilePhotoFileID := "provider-photo-file-id"
	if counterpartRole == conversation.SenderConsumer {
		counterpartID = 10
		counterpartName = "Ana"
		counterpartSurname = "Pérez"
		counterpartCategory = ""
		profilePhotoFileID = ""
	}

	return &readmodel.ConversationDetail{
		ID:     1,
		Type:   conversation.TypeWork,
		Status: conversation.StatusPending,
		Work: &readmodel.WorkConversationDetail{
			Counterpart: readmodel.ConversationParticipant{
				ID:                 counterpartID,
				Role:               counterpartRole,
				Name:               counterpartName,
				Surname:            counterpartSurname,
				CategoryName:       counterpartCategory,
				ProfilePhotoFileID: profilePhotoFileID,
			},
		},
		UpdatedOn: now,
		Messages: []readmodel.MessageDetail{
			{
				ID:         1,
				SenderRole: conversation.SenderConsumer,
				Content:    "Hola, necesito un presupuesto",
				CreatedOn:  now,
			},
		},
	}
}

func conversationFixture() conversation.Conversation {
	now := time.Now()
	fixture := &conversation.WorkConversation{
		BaseConversation: &conversation.BaseConversation{
			ID:        1,
			Type:      conversation.TypeWork,
			Status:    conversation.StatusPending,
			UpdatedOn: now,
		},
		ConsumerID: 10,
		ProviderID: 20,
	}
	fixture.SetMessages([]conversation.Message{
		{
			ID:             1,
			ConversationID: 1,
			SenderRole:     conversation.SenderConsumer,
			Content:        "Hola, necesito un presupuesto",
			CreatedOn:      now,
		},
	})

	return fixture
}

func activeConversationFixture() conversation.Conversation {
	fixture := conversationFixture()
	fixture.Base().Status = "active"
	return fixture
}

func TestServiceListsChatbotConversationsForConsumer(t *testing.T) {
	reader := &conversationReaderMock{
		chatbotSummaries: []readmodel.ConversationSummary{
			{ID: 42, Type: conversation.TypeChatbot, Status: conversation.StatusActive, Chatbot: &readmodel.ChatbotConversationSummary{Title: "Pérdida de agua en la cocina"}},
		},
	}
	service := conversation.NewService(&conversationRepositoryMock{}, &consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}, reader, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListChatbotConversations(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, 42, summaries[0].ID)
	assert.Equal(t, 10, reader.requestedParticipantID)
	assert.Equal(t, conversation.SenderConsumer, reader.requestedParticipantRole)
	assert.Equal(t, conversation.TypeChatbot, reader.requestedConversationType)
}

func TestServiceRejectsChatbotConversationListForNonConsumer(t *testing.T) {
	service := conversation.NewService(&conversationRepositoryMock{}, &consumerIDFinderMock{err: errors.New("not found")}, &providerIDFinderMock{providerID: 99}, &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListChatbotConversations(context.Background(), "auth0|provider")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanListChatbotConversations)
	assert.Nil(t, summaries)
}
