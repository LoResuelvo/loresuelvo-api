package conversation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conversationRepositoryMock struct {
	savedConversation  conversation.Conversation
	savedMessage       conversation.Message
	addedMessage       conversation.Message
	savedResult        conversation.Conversation
	addedResult        *conversation.Message
	savedChatbot       conversation.Conversation
	savedChatbotResult conversation.Conversation
	existsValue        bool
	existsCalled       bool
	saveCalled         bool
	addMessageCalled   bool
	findByIDCalled     bool
	countCalled        bool
	foundResult        conversation.Conversation
	countResult        int
	err                error
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

type providerExistenceCheckerMock struct {
	exists bool
	err    error
}

func (m *providerExistenceCheckerMock) ExistsByID(id int) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.exists, nil
}

type providerIDFinderMock struct {
	providerID int
	err        error
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

type conversationReaderMock struct {
	consumerSummaries []readmodel.ConversationSummary
	providerSummaries []readmodel.ConversationSummary
	consumerDetail    *readmodel.ConversationDetail
	providerDetail    *readmodel.ConversationDetail
	err               error
}

func (m *conversationReaderMock) FindSummariesByConsumerID(ctx context.Context, consumerID int) ([]readmodel.ConversationSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.consumerSummaries, nil
}

func (m *conversationReaderMock) FindSummariesByProviderID(ctx context.Context, providerID int) ([]readmodel.ConversationSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.providerSummaries, nil
}

func (m *conversationReaderMock) FindDetailByIDForConsumer(ctx context.Context, conversationID int) (*readmodel.ConversationDetail, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.consumerDetail, nil
}

func (m *conversationReaderMock) FindDetailByIDForProvider(ctx context.Context, conversationID int) (*readmodel.ConversationDetail, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.providerDetail, nil
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

func (m *messagePublisherMock) PublishChatbotMessage(ctx context.Context, conv conversation.Conversation, msg conversation.Message) {
	m.publishCalled = true
	m.publishedConversation = conv
	m.publishedMessage = msg
}

type chatbotMock struct {
	response *conversation.ChatbotResponse
	called   bool
	prompt   string
	err      error
}

func (m *chatbotMock) GetResponse(ctx context.Context, prompt string) (*conversation.ChatbotResponse, error) {
	m.called = true
	m.prompt = prompt
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestStartWorkRequestCreatesPendingConversationWithInitialMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "  Hola, necesito un presupuesto  ")

	require.NoError(t, err)
	require.NotNil(t, createdConversation)
	assert.Equal(t, 1, createdConversation.Base().ID)
	assert.True(t, repo.existsCalled)
	assert.True(t, repo.saveCalled)
	savedWorkConversation := repo.savedConversation.(*conversation.WorkConversation)
	assert.Equal(t, 10, savedWorkConversation.ConsumerID)
	assert.Equal(t, 20, savedWorkConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, savedWorkConversation.Base().Status)
	assert.Equal(t, conversation.SenderConsumer, repo.savedMessage.SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", repo.savedMessage.Content)
	require.Len(t, createdConversation.Messages(), 1)
	assert.Equal(t, conversation.SenderConsumer, createdConversation.Messages()[0].SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", createdConversation.Messages()[0].Content)
}

func TestCreateChatbotConversationCreatesActiveConversationWithConsumerAndChatbotMessages(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{
		Title:   "Pérdida de agua en la cocina",
		Content: "Cerrá la llave de paso y revisá el sifón.",
	}}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, &providerExistenceCheckerMock{}, &providerIDFinderMock{}, &conversationReaderMock{}, publisher, chatbot)

	createdConversation, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "  Tengo una pérdida de agua en la cocina  ")

	require.NoError(t, err)
	require.NotNil(t, createdConversation)
	assert.True(t, chatbot.called)
	assert.Equal(t, "Tengo una pérdida de agua en la cocina", chatbot.prompt)
	assert.True(t, repo.saveCalled)
	savedChatbotConversation := repo.savedChatbot.(*conversation.ChatBotConversation)
	assert.Equal(t, conversation.TypeChatbot, savedChatbotConversation.ConversationType())
	assert.Equal(t, 10, savedChatbotConversation.ConsumerID)
	assert.Equal(t, "Pérdida de agua en la cocina", savedChatbotConversation.Title)
	assert.Equal(t, conversation.StatusActive, savedChatbotConversation.Base().Status)
	require.Len(t, savedChatbotConversation.Messages(), 2)
	assert.Equal(t, conversation.SenderConsumer, savedChatbotConversation.Messages()[0].SenderRole)
	assert.Equal(t, "Tengo una pérdida de agua en la cocina", savedChatbotConversation.Messages()[0].Content)
	assert.Equal(t, conversation.SenderChatbot, savedChatbotConversation.Messages()[1].SenderRole)
	assert.Equal(t, "Cerrá la llave de paso y revisá el sifón.", savedChatbotConversation.Messages()[1].Content)
	assert.True(t, publisher.publishCalled)
	assert.Equal(t, conversation.SenderChatbot, publisher.publishedMessage.SenderRole)
}

func TestCreateChatbotConversationRejectsNonConsumerUser(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Title: "Consulta", Content: "Respuesta"}}

	service := conversation.NewService(repo, consumerIDFinder, &providerExistenceCheckerMock{}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot)

	createdConversation, err := service.CreateChatbotConversation(context.Background(), "auth0|provider", "Tengo una pérdida de agua")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanMessageChatbot)
	assert.Nil(t, createdConversation)
	assert.False(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestCreateChatbotConversationRejectsEmptyMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Title: "Consulta", Content: "Respuesta"}}

	service := conversation.NewService(repo, consumerIDFinder, &providerExistenceCheckerMock{}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot)

	createdConversation, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestCreateChatbotConversationRejectsOutOfScopeQuestionBeforeCallingChatbot(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{response: &conversation.ChatbotResponse{Title: "Consulta", Content: "Respuesta"}}

	service := conversation.NewService(repo, consumerIDFinder, &providerExistenceCheckerMock{}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot)

	createdConversation, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "¿Qué equipo ganó el último partido de fútbol?")

	assert.ErrorIs(t, err, conversation.ErrChatbotQuestionOutOfScope)
	assert.Nil(t, createdConversation)
	assert.False(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestCreateChatbotConversationReturnsChatbotErrors(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	chatbot := &chatbotMock{err: errors.New("chatbot unavailable")}

	service := conversation.NewService(repo, consumerIDFinder, &providerExistenceCheckerMock{}, &providerIDFinderMock{}, &conversationReaderMock{}, &messagePublisherMock{}, chatbot)

	createdConversation, err := service.CreateChatbotConversation(context.Background(), "auth0|consumer", "Tengo una pérdida de agua")

	assert.Error(t, err)
	assert.Nil(t, createdConversation)
	assert.True(t, chatbot.called)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsNonConsumerUser(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|provider", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanStartWorkRequest)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsNonExistingProvider(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: false}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrProviderDoesNotExist)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsMissingProviderID(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 0, "Hola")

	assert.ErrorIs(t, err, conversation.ErrProviderRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsEmptyMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsDuplicateConversation(t *testing.T) {
	repo := &conversationRepositoryMock{existsValue: true}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrAlreadyExists)
	assert.Nil(t, createdConversation)
	assert.True(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsWhenProviderExistenceCheckerFails(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{err: errors.New("database error")}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.Error(t, err)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsWhenConversationExistenceCheckFails(t *testing.T) {
	repo := &conversationRepositoryMock{err: errors.New("database error")}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.Error(t, err)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsWhenMessageCreationFails(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.saveCalled)
}

func TestGetByIDReturnsConversationDetailForParticipantConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	conversationReader := &conversationReaderMock{consumerDetail: conversationDetailFixture(conversation.SenderProvider)}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader, nil)

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 1, foundConversation.ID)
	assert.Equal(t, 20, foundConversation.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, foundConversation.Counterpart.Role)
	assert.Len(t, foundConversation.Messages, 1)
}

func TestGetByIDReturnsConversationDetailForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	conversationReader := &conversationReaderMock{providerDetail: conversationDetailFixture(conversation.SenderConsumer)}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader, nil)

	foundConversation, err := service.GetByID(context.Background(), "auth0|provider", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 10, foundConversation.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, foundConversation.Counterpart.Role)
}

func TestGetByIDRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 888}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	foundConversation, err := service.GetByID(context.Background(), "auth0|other", 1)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestGetByIDReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestSendMessageAddsConsumerMessageForParticipantConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "  ¿El jueves te queda cómodo?  ")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.True(t, repo.addMessageCalled)
	assert.Equal(t, conversation.SenderConsumer, repo.addedMessage.SenderRole)
	assert.Equal(t, "¿El jueves te queda cómodo?", repo.addedMessage.Content)
	assert.Equal(t, 1, sentMessage.ConversationID)
}

func TestSendMessageAddsProviderMessageForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: activeConversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "Sí, puedo pasar el jueves a las 10")

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
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|provider", 1, "Sí, puedo pasar el jueves a las 10")

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
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Otro detalle")

	assert.ErrorIs(t, err, conversation.ErrPendingConversationMessageLimitReached)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.countCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 888}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|other", 1, "Hola")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 999, "Hola")

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
}

func TestSendMessageRejectsEmptyContent(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, publisher)

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
	assert.False(t, publisher.publishCalled)
}

func TestSendMessagePublishesMessageAfterSuccessfulPersistence(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, publisher)

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Hola proveedor")

	require.NoError(t, err)
	require.NotNil(t, sentMessage)
	assert.True(t, publisher.publishCalled)
	assert.Equal(t, 1, publisher.publishedConversation.Base().ID)
	assert.Equal(t, "auth0|consumer", publisher.publishedSenderAuthID)
}

func TestSendMessageDoesNotPublishWhenPersistFails(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture(), err: errors.New("database error")}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	publisher := &messagePublisherMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, publisher)

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "Hola")

	assert.Error(t, err)
	assert.Nil(t, sentMessage)
	assert.False(t, publisher.publishCalled)
}

func TestListReturnsConversationSummariesForConsumer(t *testing.T) {
	now := time.Now()
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	conversationReader := &conversationReaderMock{
		consumerSummaries: []readmodel.ConversationSummary{
			{
				ID:     1,
				Status: conversation.StatusPending,
				Counterpart: readmodel.ConversationParticipant{
					ID:           20,
					Role:         conversation.SenderProvider,
					Name:         "Juan",
					Surname:      "Gómez",
					CategoryName: "Plomería",
				},
				LastMessage: &readmodel.MessageSummary{ID: 1, SenderRole: conversation.SenderConsumer, Content: "Hola", CreatedOn: now},
				UpdatedOn:   now,
			},
		},
	}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader, nil)

	summaries, err := service.List(context.Background(), "auth0|consumer")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, 1, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, 20, summaries[0].Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, summaries[0].Counterpart.Role)
	assert.Equal(t, "Juan", summaries[0].Counterpart.Name)
	assert.Equal(t, "Gómez", summaries[0].Counterpart.Surname)
	assert.Equal(t, "Plomería", summaries[0].Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, "Hola", summaries[0].LastMessage.Content)
}

func TestListReturnsConversationSummariesForProvider(t *testing.T) {
	now := time.Now()
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	conversationReader := &conversationReaderMock{
		providerSummaries: []readmodel.ConversationSummary{
			{
				ID:     1,
				Status: conversation.StatusPending,
				Counterpart: readmodel.ConversationParticipant{
					ID:      10,
					Role:    conversation.SenderConsumer,
					Name:    "Ana",
					Surname: "Pérez",
				},
				LastMessage: &readmodel.MessageSummary{ID: 1, SenderRole: conversation.SenderConsumer, Content: "Hola", CreatedOn: now},
				UpdatedOn:   now,
			},
		},
	}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader, nil)

	summaries, err := service.List(context.Background(), "auth0|provider")

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, 1, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, 10, summaries[0].Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].Counterpart.Role)
	assert.Equal(t, "Ana", summaries[0].Counterpart.Name)
	assert.Equal(t, "Pérez", summaries[0].Counterpart.Surname)
	assert.Empty(t, summaries[0].Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
}

func TestListRejectsAuthenticatedUserWithoutParticipantProfile(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{}, &messagePublisherMock{})

	summaries, err := service.List(context.Background(), "auth0|unknown")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, summaries)
}

func conversationDetailFixture(counterpartRole string) *readmodel.ConversationDetail {
	now := time.Now()
	counterpartID := 20
	counterpartName := "Juan"
	counterpartSurname := "Gómez"
	counterpartCategory := "Plomería"
	if counterpartRole == conversation.SenderConsumer {
		counterpartID = 10
		counterpartName = "Ana"
		counterpartSurname = "Pérez"
		counterpartCategory = ""
	}

	return &readmodel.ConversationDetail{
		ID:     1,
		Status: conversation.StatusPending,
		Counterpart: readmodel.ConversationParticipant{
			ID:           counterpartID,
			Role:         counterpartRole,
			Name:         counterpartName,
			Surname:      counterpartSurname,
			CategoryName: counterpartCategory,
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
