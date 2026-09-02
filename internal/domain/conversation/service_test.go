package conversation_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	domainclock "github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
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
	savedResult            conversation.Conversation
	savedChatbot           conversation.Conversation
	savedChatbotResult     conversation.Conversation
	updatedConversation    conversation.Conversation
	existsValue            bool
	existsCalled           bool
	saveCalled             bool
	updateCalled           bool
	updateCalls            int
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
	if c.ID() > 0 {
		r.updateCalled = true
		r.updateCalls++
		r.updatedConversation = c
	} else {
		r.saveCalled = true
	}
	r.savedConversation = c
	if c.ConversationType() == conversation.TypeChatbot {
		r.savedChatbot = c
	}
	if chatbotConversation, ok := c.(*conversation.ChatBotConversation); ok && c.ID() > 0 {
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
	if r.savedResult != nil {
		return r.savedResult, nil
	}
	if r.savedChatbotResult != nil {
		return r.savedChatbotResult, nil
	}
	if c.ID() == 0 {
		c.SetPersistenceState(1, time.Time{})
	}
	messages := c.Messages()
	for index := range messages {
		if messages[index].ID == 0 {
			messages[index].ID = index + 1
			messages[index].ConversationID = c.ID()
			messages[index].CreatedOn = time.Now()
		}
	}
	c.SetMessages(messages)
	if len(messages) > 0 {
		r.savedMessage = messages[len(messages)-1]
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
	providerID                          int
	err                                 error
	providers                           []provider.Provider
	requestedCategoryID                 int
	requestedCoverageZoneID             int
	findByCategoryAndCoverageZoneCalled bool
	findByCategoryAndCoverageZoneErr    error
}

type userRepositoryMock struct {
	consumer          *consumerIDFinderMock
	provider          *providerIDFinderMock
	findByAuthIDCalls int
}

func newUserRepositoryMock(consumer *consumerIDFinderMock, provider *providerIDFinderMock) *userRepositoryMock {
	return &userRepositoryMock{consumer: consumer, provider: provider}
}

func (m *userRepositoryMock) FindByAuthID(authID string) (user.User, error) {
	m.findByAuthIDCalls++
	if m.consumer != nil {
		if id, err := m.consumer.FindIDByAuthID(authID); err == nil {
			return consumer.RehydrateConsumer(
				user.RehydrateBaseUser(id, authID, "consumer@example.com", "", "", conversation.SenderConsumer, nil),
				consumer.Address{Street: "Av. Rivadavia", StreetNumber: "5100"},
				consumer.GeoPoint{},
				coveragezone.CoverageZone{ID: 6, Name: "Comuna 6", Enabled: true},
			), nil
		}
	}
	if m.provider != nil {
		id, err := m.provider.FindIDByAuthID(authID)
		if err == nil {
			return user.RehydrateBaseUser(id, authID, "", "", "", conversation.SenderProvider, nil), nil
		}
		return nil, err
	}
	return nil, errors.New("user not found")
}

func (m *userRepositoryMock) FindIDByAuthID(authID string) (int, error) {
	found, err := m.FindByAuthID(authID)
	if err != nil {
		return 0, err
	}
	return found.ID(), nil
}

func (m *userRepositoryMock) FindAuthIDByID(id int) (string, error) { return "", nil }

func (m *userRepositoryMock) FindConsumerByAuthID(_ context.Context, authID string) (*consumer.Consumer, error) {
	foundUser, err := m.FindByAuthID(authID)
	if err != nil {
		return nil, err
	}
	foundConsumer, ok := foundUser.(*consumer.Consumer)
	if !ok {
		return nil, errors.New("user is not a consumer")
	}

	return foundConsumer, nil
}

func (m *userRepositoryMock) FindProviderByID(ctx context.Context, providerID int) (*provider.Provider, error) {
	if m.provider == nil {
		return nil, errors.New("provider not found")
	}
	for index := range m.provider.providers {
		if m.provider.providers[index].ID() == providerID {
			found := m.provider.providers[index]
			return &found, nil
		}
	}
	return nil, errors.New("provider not found")
}

func (m *userRepositoryMock) FindProvidersByCategoryAndCoverageZoneID(ctx context.Context, categoryID, coverageZoneID int) ([]provider.Provider, error) {
	return m.provider.FindProvidersByCategoryAndCoverageZoneID(ctx, categoryID, coverageZoneID)
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

func (m *providerIDFinderMock) FindProviderByID(_ context.Context, providerID int) (*provider.Provider, error) {
	if m.err != nil {
		return nil, m.err
	}
	for index := range m.providers {
		if m.providers[index].ID() == providerID {
			found := m.providers[index]
			return &found, nil
		}
	}
	return nil, errors.New("provider not found")
}

func (m *providerIDFinderMock) FindProvidersByCategoryAndCoverageZoneID(_ context.Context, categoryID, coverageZoneID int) ([]provider.Provider, error) {
	m.findByCategoryAndCoverageZoneCalled = true
	m.requestedCategoryID = categoryID
	m.requestedCoverageZoneID = coverageZoneID
	if m.findByCategoryAndCoverageZoneErr != nil {
		return nil, m.findByCategoryAndCoverageZoneErr
	}

	eligibleProviders := make([]provider.Provider, 0, len(m.providers))
	for _, foundProvider := range m.providers {
		if foundProvider.Category == nil || foundProvider.Category.ID != categoryID {
			continue
		}
		for _, zone := range foundProvider.CoverageZones {
			if zone.ID == coverageZoneID && zone.Enabled {
				eligibleProviders = append(eligibleProviders, foundProvider)
				break
			}
		}
	}

	return eligibleProviders, nil
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

func (m *conversationReaderMock) FindSummariesByUserAndType(ctx context.Context, foundUser user.User, conversationType string) ([]readmodel.ConversationSummary, error) {
	return m.FindSummariesByParticipantIDRoleAndType(ctx, foundUser.ID(), foundUser.Role(), conversationType)
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
		files = append(files, filedomain.MessageImage{Image: filedomain.Image{FileID: fileID, OriginalName: fileID + ".jpg", URL: "https://files/" + fileID}})
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
			MessageImage: filedomain.MessageImage{Image: filedomain.Image{FileID: fileID, OriginalName: fileID + ".jpg", URL: "https://files/" + fileID}},
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
		files[fileID] = filedomain.MessageImage{Image: filedomain.Image{FileID: fileID, OriginalName: fileID + ".jpg", URL: "https://files/" + fileID}}
	}
	return files, nil
}

func (m *fileURLResolverMock) PrepareMessageAudio(_ context.Context, _ string, fileID string) (*filedomain.MessageAudio, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &filedomain.MessageAudio{
		FileID:          fileID,
		OriginalName:    fileID + ".webm",
		URL:             "https://files/" + fileID,
		MimeType:        "audio/webm",
		Codec:           "opus",
		DurationSeconds: 18,
	}, nil
}

func (m *fileURLResolverMock) ResolveMessageAudios(_ context.Context, fileIDs []string) (map[string]filedomain.MessageAudio, error) {
	if m.err != nil {
		return nil, m.err
	}
	audios := make(map[string]filedomain.MessageAudio, len(fileIDs))
	for _, fileID := range fileIDs {
		audios[fileID] = filedomain.MessageAudio{
			FileID:          fileID,
			OriginalName:    fileID + ".webm",
			URL:             "https://files/" + fileID,
			MimeType:        "audio/webm",
			Codec:           "opus",
			DurationSeconds: 18,
		}
	}
	return audios, nil
}

func (m *fileURLResolverMock) PrepareMessageVideo(_ context.Context, _ string, fileID string) (*filedomain.MessageVideo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &filedomain.MessageVideo{
		FileID:          fileID,
		OriginalName:    fileID + ".mp4",
		URL:             "https://files/" + fileID,
		MimeType:        "video/mp4",
		VideoCodec:      "h264",
		AudioCodec:      "aac",
		DurationSeconds: 18,
		Width:           1080,
		Height:          1920,
	}, nil
}

func (m *fileURLResolverMock) ResolveMessageVideos(_ context.Context, fileIDs []string) (map[string]filedomain.MessageVideo, error) {
	if m.err != nil {
		return nil, m.err
	}
	videos := make(map[string]filedomain.MessageVideo, len(fileIDs))
	for _, fileID := range fileIDs {
		videos[fileID] = filedomain.MessageVideo{
			FileID:          fileID,
			OriginalName:    fileID + ".mp4",
			URL:             "https://files/" + fileID,
			MimeType:        "video/mp4",
			VideoCodec:      "h264",
			AudioCodec:      "aac",
			DurationSeconds: 18,
			Width:           1080,
			Height:          1920,
		}
	}
	return videos, nil
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
	rankingCalled       bool
	rankingRequest      conversation.ProviderRankingRequest
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

func (m *chatbotMock) RankProviders(_ context.Context, request conversation.ProviderRankingRequest) (*conversation.ProviderRankingResponse, error) {
	m.rankingCalled = true
	m.rankingRequest = request
	if m.err != nil {
		return nil, m.err
	}
	recommendations := make([]conversation.ProviderRankingRecommendation, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		recommendations = append(recommendations, conversation.ProviderRankingRecommendation{
			Reference: candidate.Reference,
			Reason:    "Razón de prueba.",
		})
	}
	return &conversation.ProviderRankingResponse{Recommendations: recommendations}, nil
}

type workOrderReaderMock struct {
	ratingStatsByProviderID     map[int]provider.RatingStats
	paidWorkHistoryByProviderID map[int][]providerreadmodel.WorkOrder
	ratingStatsErr              error
	paidWorkHistoryErr          error
}

func (reader *workOrderReaderMock) FindRatingStatsByProviderIDs(_ context.Context, _ []int) (map[int]provider.RatingStats, error) {
	if reader.ratingStatsErr != nil {
		return nil, reader.ratingStatsErr
	}
	if reader.ratingStatsByProviderID == nil {
		return map[int]provider.RatingStats{}, nil
	}
	return reader.ratingStatsByProviderID, nil
}

func (reader *workOrderReaderMock) FindPaidWorkHistoryByProviderIDs(_ context.Context, _ []int) (map[int][]providerreadmodel.WorkOrder, error) {
	if reader.paidWorkHistoryErr != nil {
		return nil, reader.paidWorkHistoryErr
	}
	if reader.paidWorkHistoryByProviderID == nil {
		return map[int][]providerreadmodel.WorkOrder{}, nil
	}
	return reader.paidWorkHistoryByProviderID, nil
}

func newConversationService(
	conversationRepository conversation.Repository,
	userRepository conversation.UserRepository,
	conversationReader conversation.Reader,
	messagePublisher conversation.MessagePublisher,
	chatbot conversation.Chatbot,
	categoryRepository conversation.RecommendationCategoryLister,
	fileService conversation.FileService,
	clock domainclock.Clock,
) *conversation.Service {
	return conversation.NewService(
		conversationRepository,
		userRepository,
		conversationReader,
		messagePublisher,
		chatbot,
		categoryRepository,
		fileService,
		clock,
		conversation.DefaultProviderRecommendationConfig(),
		&workOrderReaderMock{},
	)
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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, publisher, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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
	assert.Equal(t, conversation.StatusActive, savedChatbotConversation.Status())
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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, fileService, fixedClock{now: time.Now().UTC()})

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
	assert.Equal(t, []filedomain.MessageImage{{Image: filedomain.Image{FileID: "image-file-id", OriginalName: "image-file-id.jpg", URL: "https://files/image-file-id"}, Description: "Descripción de prueba"}}, savedChatbotConversation.Messages()[0].Images)
}

func TestCreateChatbotConversationIncludesRecommendedProvidersWhenDiagnosisIsCompleted(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	plumbingCategory := &category.Category{ID: 3, Name: "Plomería", NormalizedName: "plomería"}
	recommendedProvider, err := provider.NewProvider("auth0|provider", "juan@example.com", "Juan", "Gómez", plumbingCategory, &filedomain.Image{FileID: "provider-photo-file-id"}, []coveragezone.CoverageZone{{ID: 6, Name: "Comuna 6", Enabled: true}})
	require.NoError(t, err)
	recommendedProvider.SetPersistenceID(20)
	outsideProvider, err := provider.NewProvider("auth0|outside-provider", "pedro@example.com", "Pedro", "López", plumbingCategory, nil, []coveragezone.CoverageZone{{ID: 14, Name: "Comuna 14", Enabled: true}})
	require.NoError(t, err)
	outsideProvider.SetPersistenceID(21)
	electricityCategory := &category.Category{ID: 4, Name: "Electricidad", NormalizedName: "electricidad"}
	otherCategoryProvider, err := provider.NewProvider("auth0|electrician", "laura@example.com", "Laura", "Suárez", electricityCategory, nil, []coveragezone.CoverageZone{{ID: 6, Name: "Comuna 6", Enabled: true}})
	require.NoError(t, err)
	otherCategoryProvider.SetPersistenceID(22)
	categoryLister := &recommendationCategoryListerMock{categories: []category.Category{*plumbingCategory}}
	providerFinder := &providerIDFinderMock{providers: []provider.Provider{*recommendedProvider, *outsideProvider, *otherCategoryProvider}}
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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerFinder), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, categoryLister, fileURLResolver, fixedClock{now: time.Now().UTC()})

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
	assert.True(t, providerFinder.findByCategoryAndCoverageZoneCalled)
	assert.Equal(t, 3, providerFinder.requestedCategoryID)
	assert.Equal(t, 6, providerFinder.requestedCoverageZoneID)
	assert.Equal(t, []string{"provider-photo-file-id"}, fileURLResolver.resolvedFileIDs)
	require.Len(t, createdResult.RecommendedProviders, 1)
	assert.Equal(t, 20, createdResult.RecommendedProviders[0].ID())
	assert.Equal(t, "Juan", createdResult.RecommendedProviders[0].Name())
	assert.Equal(t, "Gómez", createdResult.RecommendedProviders[0].Surname())
	assert.Equal(t, "Plomería", createdResult.RecommendedProviders[0].Category.Name)
	assert.Equal(t, "https://cdn/provider.jpg", createdResult.RecommendedProviders[0].ProfilePhoto().URL)
}

func TestCreateChatbotConversationBuildsRecommendationEvidenceFromWorkOrderReads(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	plumbingCategory := &category.Category{ID: 3, Name: "Plomería", NormalizedName: "plomería"}
	recommendedProvider, err := provider.NewProvider(
		"auth0|provider",
		"juan@example.com",
		"Juan",
		"Gómez",
		plumbingCategory,
		nil,
		[]coveragezone.CoverageZone{{ID: 6, Name: "Comuna 6", Enabled: true}},
	)
	require.NoError(t, err)
	recommendedProvider.SetPersistenceID(20)
	providerFinder := &providerIDFinderMock{providers: []provider.Provider{*recommendedProvider}}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Status:  conversation.ChatbotResponseAnswered,
		Title:   "Pérdida de agua",
		Content: "Contactá a un plomero.",
		Assessment: conversation.ChatbotAssessmentResponse{
			Action:              conversation.ChatbotAssessmentReplace,
			Outcome:             conversation.AssessmentProfessionalRequired,
			ProblemTitle:        "Pérdida de agua",
			ProblemDescription:  "Pierde agua la bacha.",
			ProblemCategoryName: "Plomería",
		},
	}}
	recentWork := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	workOrderReader := &workOrderReaderMock{
		ratingStatsByProviderID: map[int]provider.RatingStats{
			20: {Total: 9, Count: 2, Distribution: provider.RatingDistribution{0, 0, 0, 1, 1}},
		},
		paidWorkHistoryByProviderID: map[int][]providerreadmodel.WorkOrder{
			20: {{
				ID:          84,
				ScheduledOn: recentWork,
				Description: "Reparación de sifón",
				Status:      "paid",
				CompletionReport: &providerreadmodel.CompletionReport{
					Description: "Trabajo terminado.",
					ReportedOn:  recentWork.Add(time.Hour),
				},
				Review: &providerreadmodel.Review{Rating: 5, Description: "Resolvió la pérdida."},
			}},
		},
	}
	service := conversation.NewService(
		repo,
		newUserRepositoryMock(consumerIDFinder, providerFinder),
		&conversationReaderMock{},
		&messagePublisherMock{},
		chatbot,
		&recommendationCategoryListerMock{categories: []category.Category{*plumbingCategory}},
		&fileURLResolverMock{},
		fixedClock{now: recentWork},
		conversation.DefaultProviderRecommendationConfig(),
		workOrderReader,
	)

	_, err = service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Pierde agua la bacha")

	require.NoError(t, err)
	assert.True(t, chatbot.rankingCalled)
	require.Len(t, chatbot.rankingRequest.Candidates, 1)
	evidence := chatbot.rankingRequest.Candidates[0].Evidence
	assert.Equal(t, 20, chatbot.rankingRequest.Candidates[0].ProviderID)
	assert.Equal(t, 4.5, evidence.RatingAverage)
	assert.Equal(t, 2, evidence.RatingCount)
	assert.Equal(t, provider.RatingDistribution{0, 0, 0, 1, 1}, evidence.RatingDistribution)
	assert.Equal(t, 1, evidence.PaidWorkCount)
	assert.Equal(t, recentWork, evidence.MostRecentPaidWork)
	assert.Equal(t, "Resolvió la pérdida.", evidence.WorkHistory[0].Review.Description)
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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerFinder), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, categoryLister, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Tengo humedad")

	require.NoError(t, err)
	require.NotNil(t, createdResult)
	require.NotNil(t, createdResult.Assessment)
	assert.Equal(t, conversation.AssessmentCollectingInformation, createdResult.Assessment.Outcome)
	assert.Nil(t, createdResult.ProblemCategory)
	assert.Empty(t, createdResult.RecommendedProviders)
	assert.False(t, providerFinder.findByCategoryAndCoverageZoneCalled)
	assert.True(t, categoryLister.called)
	assert.Equal(t, []string{"Plomería"}, categoryNames(chatbot.availableCategories))
}

func TestCreateChatbotConversationRejectsNonConsumerUser(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Status: conversation.ChatbotResponseAnswered, Title: "Consulta", Content: "Respuesta"}}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	createdResult, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Tengo una pérdida de agua")

	assert.Error(t, err)
	assert.Nil(t, createdResult)
	assert.True(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestContinueChatbotConversationAddsConsumerAndChatbotMessagesToExistingChatbotConversation(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: &conversation.ChatBotConversation{
			BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
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
	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, publisher, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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
			BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
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
	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|consumer", 7, "Saqué una foto", []string{"detail-image-id"})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, chatbot.question.Images, 1)
	assert.Equal(t, "detail-image-id", chatbot.question.Images[0].FileID)
	require.Len(t, result.Conversation.Messages(), 2)
	assert.Equal(t, []filedomain.MessageImage{{Image: filedomain.Image{FileID: "detail-image-id", OriginalName: "detail-image-id.jpg", URL: "https://files/detail-image-id"}, Description: "Descripción de prueba"}}, result.Conversation.Messages()[0].Images)
}

func TestContinueChatbotConversationSummarizesPendingContextWhenRecentMessageLimitIsReached(t *testing.T) {
	chatbotConversation := &conversation.ChatBotConversation{
		BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
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
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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
		BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
		ConsumerID:       10,
		Title:            "Pérdida de agua",
	}}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 99}, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	result, err := service.ContinueChatbotConversation(context.Background(), "auth0|other", 7, "Hola")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, result)
	assert.False(t, repo.startProcessingCalled)
	assert.False(t, repo.updateCalled)
}

func TestContinueChatbotConversationReturnsProcessingConflict(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: &conversation.ChatBotConversation{
			BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
			ConsumerID:       10,
			Title:            "Pérdida de agua",
		},
		startProcessingErr: conversation.ErrChatbotConversationAlreadyProcessing,
	}
	chatbot := &chatbotMock{}
	now := time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC)
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, chatbot, &recommendationCategoryListerMock{}, &fileURLResolverMock{}, fixedClock{now: now})

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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: time.Now().UTC()})

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

	userRepository := newUserRepositoryMock(consumerIDFinder, providerIDFinder)
	service := newConversationService(repo, userRepository, conversationReader, nil, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|provider", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	require.NotNil(t, foundConversation.Work)
	assert.Equal(t, 10, foundConversation.Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, foundConversation.Work.Counterpart.Role)
	assert.Equal(t, 1, userRepository.findByAuthIDCalls)
	assert.Equal(t, conversation.SenderProvider, conversationReader.requestedParticipantRole)
	assert.Equal(t, conversation.TypeWork, conversationReader.requestedConversationType)
}

func TestGetByIDReturnsChatbotConversationDetailForOwnerConsumer(t *testing.T) {
	recommendedCategoryID := 3
	repo := &conversationRepositoryMock{foundResult: &conversation.ChatBotConversation{
		BaseConversation:   conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
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
	recommendedProvider, err := provider.NewProvider("auth0|provider", "juan@example.com", "Juan", "Gómez", plumbingCategory, &filedomain.Image{FileID: "provider-photo-file-id"}, []coveragezone.CoverageZone{{ID: 6, Name: "Comuna 6", Enabled: true}})
	require.NoError(t, err)
	recommendedProvider.SetPersistenceID(20)
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
	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerFinder), conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 7)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 7, conversationReader.requestedConversationID)
	assert.Equal(t, conversation.SenderConsumer, conversationReader.requestedParticipantRole)
	assert.Equal(t, conversation.TypeChatbot, conversationReader.requestedConversationType)
	assert.True(t, providerFinder.findByCategoryAndCoverageZoneCalled)
	assert.Equal(t, recommendedCategoryID, providerFinder.requestedCategoryID)
	assert.Equal(t, 6, providerFinder.requestedCoverageZoneID)
	require.NotNil(t, foundConversation.Chatbot)
	require.Len(t, foundConversation.Chatbot.RecommendedProviders, 1)
	assert.Equal(t, 20, foundConversation.Chatbot.RecommendedProviders[0].ID())
	assert.Equal(t, "https://cdn/provider.jpg", foundConversation.Chatbot.RecommendedProviders[0].ProfilePhoto().URL)
}

func TestGetByIDRejectsChatbotConversationForNonOwnerConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: &conversation.ChatBotConversation{
		BaseConversation: conversation.RehydrateBaseConversation(7, conversation.TypeChatbot, conversation.StatusActive, time.Time{}, nil),
		ConsumerID:       10,
	}}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	conversationReader := &conversationReaderMock{}
	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, &providerIDFinderMock{}), conversationReader, nil, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|other", 1)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestGetByIDReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestSendMessageAddsConsumerMessageForParticipantConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "  ¿El jueves te queda cómodo?  ", nil, "")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.True(t, repo.updateCalled)
	assert.Equal(t, conversation.SenderConsumer, repo.savedMessage.SenderRole)
	assert.Equal(t, "¿El jueves te queda cómodo?", repo.savedMessage.Content)
	assert.Equal(t, 1, sentMessage.ConversationID)
}

func TestSendMessageAddsResolvedPrivateImages(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	publisher := &messagePublisherMock{}
	fileService := &fileURLResolverMock{}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, fileService, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "", []string{"image-file-id"}, "")

	require.NoError(t, err)
	require.Len(t, sentMessage.Images, 1)
	assert.Equal(t, "image-file-id", sentMessage.Images[0].FileID)
	assert.Equal(t, "image-file-id.jpg", sentMessage.Images[0].OriginalName)
	assert.Equal(t, "https://files/image-file-id", sentMessage.Images[0].URL)
	assert.Equal(t, sentMessage.Images, publisher.publishedMessage.Images)
}

func TestSendMessageRejectsAudioCombinedWithTextOrImages(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		imageFileIDs []string
		audioFileID  string
	}{
		{name: "text", content: "mensaje", audioFileID: "audio-file-id"},
		{name: "images", imageFileIDs: []string{"image-file-id"}, audioFileID: "audio-file-id"},
		{name: "text and images", content: "mensaje", imageFileIDs: []string{"image-file-id"}, audioFileID: "audio-file-id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
			service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

			sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, test.content, test.imageFileIDs, test.audioFileID)

			assert.ErrorIs(t, err, conversation.ErrMessageAudioMustBeExclusive)
			assert.Nil(t, sentMessage)
			assert.False(t, repo.findByIDCalled)
			assert.False(t, repo.updateCalled)
		})
	}
}

func TestSendMessageAddsConsumerVideoOnly(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	publisher := &messagePublisherMock{}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "", nil, "", "video-file-id")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	require.NotNil(t, sentMessage.Video)
	assert.Equal(t, conversation.SenderConsumer, sentMessage.SenderRole)
	assert.Empty(t, sentMessage.Content)
	assert.Empty(t, sentMessage.Images)
	assert.Equal(t, "video-file-id", sentMessage.Video.FileID)
	assert.Equal(t, sentMessage.Video, publisher.publishedMessage.Video)
}

func TestSendMessageAddsProviderVideoWithText(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	publisher := &messagePublisherMock{}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{err: errors.New("consumer not found")}, &providerIDFinderMock{providerID: 20}), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "  Te muestro la reparación.  ", nil, "", "video-file-id")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	require.NotNil(t, sentMessage.Video)
	assert.Equal(t, conversation.SenderProvider, sentMessage.SenderRole)
	assert.Equal(t, "Te muestro la reparación.", sentMessage.Content)
	assert.Equal(t, "video-file-id", sentMessage.Video.FileID)
	assert.Equal(t, sentMessage.Video, publisher.publishedMessage.Video)
}

func TestSendMessageRejectsVideoCombinedWithImages(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Detalle", []string{"image-file-id"}, "", "video-file-id")

	assert.ErrorIs(t, err, conversation.ErrMessageVideoCannotIncludeImages)
	assert.Nil(t, sentMessage)
	assert.False(t, repo.findByIDCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageRejectsUnavailableImageBeforePersistence(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	fileService := &fileURLResolverMock{err: filedomain.ErrMessageImageNotAvailable}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, fileService, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Problem", []string{"image-file-id"}, "")

	assert.Nil(t, sentMessage)
	assert.ErrorIs(t, err, conversation.ErrMessageImageNotAvailable)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageAddsProviderMessageForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "Sí, puedo pasar el jueves a las 10", nil, "")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.True(t, repo.updateCalled)
	assert.Equal(t, conversation.SenderProvider, repo.savedMessage.SenderRole)
	assert.Equal(t, "Sí, puedo pasar el jueves a las 10", repo.savedMessage.Content)
	assert.Equal(t, 1, sentMessage.ConversationID)
}

func TestSendMessageAddsProviderAudioForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	publisher := &messagePublisherMock{}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{err: errors.New("consumer not found")}, &providerIDFinderMock{providerID: 20}), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "", nil, "audio-file-id")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	require.NotNil(t, sentMessage.Audio)
	assert.Equal(t, conversation.SenderProvider, repo.savedMessage.SenderRole)
	assert.Empty(t, sentMessage.Content)
	assert.Empty(t, sentMessage.Images)
	assert.Equal(t, "audio-file-id", sentMessage.Audio.FileID)
	assert.Equal(t, sentMessage.Audio, publisher.publishedMessage.Audio)
}

func TestSendMessageAddsConsumerAudioToPendingConversation(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture(), countResult: 1}
	publisher := &messagePublisherMock{}
	service := newConversationService(repo, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{err: errors.New("provider not found")}), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "", nil, "audio-file-id")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	require.NotNil(t, sentMessage.Audio)
	assert.True(t, repo.countCalled)
	assert.True(t, repo.updateCalled)
	assert.Equal(t, conversation.SenderConsumer, sentMessage.SenderRole)
	assert.Equal(t, "audio-file-id", sentMessage.Audio.FileID)
	assert.Equal(t, sentMessage.Audio, publisher.publishedMessage.Audio)
}

func TestSendMessageRejectsProviderMessageInPendingConversation(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "Sí, puedo pasar el jueves a las 10", nil, "")

	assert.ErrorIs(t, err, conversation.ErrPendingConversationRequiresAcceptance)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageRejectsProviderAudioInPendingConversation(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "", nil, "audio-file-id")

	assert.ErrorIs(t, err, conversation.ErrPendingConversationRequiresAcceptance)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageRejectsConsumerMessageWhenPendingLimitWasReached(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: conversationFixture(),
		countResult: conversation.PendingConsumerMessageLimit,
	}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Otro detalle", nil, "")

	assert.ErrorIs(t, err, conversation.ErrPendingConversationMessageLimitReached)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.countCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageRejectsConsumerAudioWhenPendingLimitWasReached(t *testing.T) {
	repo := &conversationRepositoryMock{
		foundResult: conversationFixture(),
		countResult: conversation.PendingConsumerMessageLimit,
	}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "", nil, "audio-file-id")

	assert.ErrorIs(t, err, conversation.ErrPendingConversationMessageLimitReached)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.countCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerIDFinder := &providerIDFinderMock{providerID: 888}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|other", 1, "Hola", nil, "")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 999, "Hola", nil, "")

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.updateCalled)
}

func TestSendMessageRejectsEmptyContent(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "   ", nil, "")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.updateCalled)
	assert.False(t, publisher.publishCalled)
}

func TestSendMessagePublishesMessageAfterSuccessfulPersistence(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Hola proveedor", nil, "")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, publisher.publishCalled)
	assert.Equal(t, 1, publisher.publishedConversation.ID())
	assert.Equal(t, "auth0|consumer", publisher.publishedSenderAuthID)
}

func TestSendMessageDoesNotPublishWhenPersistFails(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture(), err: errors.New("database error")}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, publisher, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Hola", nil, "")

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
	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: time.Now().UTC()})

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

func TestListResolvesAudioInLatestWorkMessageSummary(t *testing.T) {
	now := time.Now()
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	conversationReader := &conversationReaderMock{
		consumerSummaries: []readmodel.ConversationSummary{{
			ID:     1,
			Type:   conversation.TypeWork,
			Status: conversation.StatusActive,
			Work: &readmodel.WorkConversationSummary{Counterpart: readmodel.ConversationParticipant{
				ID:                 20,
				Role:               conversation.SenderProvider,
				Name:               "Juan",
				Surname:            "Gómez",
				ProfilePhotoFileID: "provider-photo-file-id",
			}},
			LastMessage: &readmodel.MessageSummary{
				ID:         2,
				SenderRole: conversation.SenderConsumer,
				Audio:      &filedomain.MessageAudio{FileID: "summary-audio-file-id"},
				CreatedOn:  now,
			},
			UpdatedOn: now,
		}},
	}

	fileURLResolver := &fileURLResolverMock{urlsByFileID: map[string]string{"provider-photo-file-id": "https://cdn/provider.jpg"}}
	service := newConversationService(&conversationRepositoryMock{}, newUserRepositoryMock(consumerIDFinder, providerIDFinder), conversationReader, nil, &chatbotMock{}, nil, fileURLResolver, fixedClock{now: now})

	summaries, err := service.ListWorkConversations(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].LastMessage)
	require.NotNil(t, summaries[0].LastMessage.Audio)
	assert.Equal(t, "https://files/summary-audio-file-id", summaries[0].LastMessage.Audio.URL)
	assert.Equal(t, "audio/webm", summaries[0].LastMessage.Audio.MimeType)
	assert.Equal(t, "opus", summaries[0].LastMessage.Audio.Codec)
	assert.Equal(t, 18, summaries[0].LastMessage.Audio.DurationSeconds)
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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), conversationReader, nil, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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

	service := newConversationService(repo, newUserRepositoryMock(consumerIDFinder, providerIDFinder), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

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
		BaseConversation: conversation.RehydrateBaseConversation(1, conversation.TypeWork, conversation.StatusPending, now, nil),
		ConsumerID:       10,
		ProviderID:       20,
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
	_ = fixture.Activate()
	return fixture
}

func TestServiceListsChatbotConversationsForConsumer(t *testing.T) {
	reader := &conversationReaderMock{
		chatbotSummaries: []readmodel.ConversationSummary{
			{ID: 42, Type: conversation.TypeChatbot, Status: conversation.StatusActive, Chatbot: &readmodel.ChatbotConversationSummary{Title: "Pérdida de agua en la cocina"}},
		},
	}
	service := newConversationService(&conversationRepositoryMock{}, newUserRepositoryMock(&consumerIDFinderMock{consumerID: 10}, &providerIDFinderMock{}), reader, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListChatbotConversations(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, 42, summaries[0].ID)
	assert.Equal(t, 10, reader.requestedParticipantID)
	assert.Equal(t, conversation.SenderConsumer, reader.requestedParticipantRole)
	assert.Equal(t, conversation.TypeChatbot, reader.requestedConversationType)
}

func TestServiceRejectsChatbotConversationListForNonConsumer(t *testing.T) {
	service := newConversationService(&conversationRepositoryMock{}, newUserRepositoryMock(&consumerIDFinderMock{err: errors.New("not found")}, &providerIDFinderMock{providerID: 99}), &conversationReaderMock{}, &messagePublisherMock{}, &chatbotMock{}, nil, &fileURLResolverMock{}, fixedClock{now: time.Now().UTC()})

	summaries, err := service.ListChatbotConversations(context.Background(), "auth0|provider")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanListChatbotConversations)
	assert.Nil(t, summaries)
}
