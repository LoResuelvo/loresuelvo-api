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
	savedConversation conversation.Conversation
	savedMessage      conversation.Message
	addedMessage      conversation.Message
	savedResult       *conversation.Conversation
	addedResult       *conversation.Message
	existsValue       bool
	existsCalled      bool
	saveCalled        bool
	addMessageCalled  bool
	findByIDCalled    bool
	saveStatusCalled  bool
	countCalled       bool
	foundResult       *conversation.Conversation
	savedStatus       conversation.Conversation
	countResult       int
	err               error
}

func (r *conversationRepositoryMock) ExistsBetween(consumerID, providerID int) (bool, error) {
	r.existsCalled = true
	if r.err != nil {
		return false, r.err
	}
	return r.existsValue, nil
}

func (r *conversationRepositoryMock) SaveWithMessage(c conversation.Conversation, m conversation.Message) (*conversation.Conversation, error) {
	r.saveCalled = true
	r.savedConversation = c
	r.savedMessage = m
	if r.err != nil {
		return nil, r.err
	}
	if r.savedResult != nil {
		return r.savedResult, nil
	}
	c.ID = 1
	m.ConversationID = c.ID
	m.ID = 1
	c.Messages = []conversation.Message{m}
	return &c, nil
}

func (r *conversationRepositoryMock) FindByID(ctx context.Context, conversationID int) (*conversation.Conversation, error) {
	r.findByIDCalled = true
	if r.err != nil {
		return nil, r.err
	}
	if r.foundResult != nil {
		return r.foundResult, nil
	}
	return nil, conversation.ErrConversationDoesNotExist
}

// func (r *conversationRepositoryMock) SaveStatus(ctx context.Context, conversation conversation.Conversation) error {
// 	r.saveStatusCalled = true
// 	r.savedStatus = conversation
// 	if r.err != nil {
// 		return r.err
// 	}
// 	return nil
// }

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

func TestStartWorkRequestCreatesPendingConversationWithInitialMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "  Hola, necesito un presupuesto  ")

	require.NoError(t, err)
	require.NotNil(t, createdConversation)
	assert.Equal(t, 1, createdConversation.ID)
	assert.True(t, repo.existsCalled)
	assert.True(t, repo.saveCalled)
	assert.Equal(t, 10, repo.savedConversation.ConsumerID)
	assert.Equal(t, 20, repo.savedConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, repo.savedConversation.Status)
	assert.Equal(t, conversation.SenderConsumer, repo.savedMessage.SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", repo.savedMessage.Content)
	require.Len(t, createdConversation.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, createdConversation.Messages[0].SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", createdConversation.Messages[0].Content)
}

func TestStartWorkRequestRejectsNonConsumerUser(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader)

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader)

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

	sentMessage, err := service.SendMessage(context.Background(), "auth0|consumer", 1, "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, sentMessage)
	assert.True(t, repo.findByIDCalled)
	assert.False(t, repo.addMessageCalled)
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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader)

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, conversationReader)

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

	service := conversation.NewService(repo, consumerIDFinder, providerExistenceChecker, providerIDFinder, &conversationReaderMock{})

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

func conversationFixture() *conversation.Conversation {
	now := time.Now()
	return &conversation.Conversation{
		ID:         1,
		ConsumerID: 10,
		ProviderID: 20,
		Status:     conversation.StatusPending,
		UpdatedOn:  now,
		Messages: []conversation.Message{
			{
				ID:             1,
				ConversationID: 1,
				SenderRole:     conversation.SenderConsumer,
				Content:        "Hola, necesito un presupuesto",
				CreatedOn:      now,
			},
		},
	}
}

func activeConversationFixture() *conversation.Conversation {
	fixture := conversationFixture()
	fixture.Status = "active"
	return fixture
}
