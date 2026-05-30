package conversation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conversationRepositoryMock struct {
	savedConversation conversation.Conversation
	savedMessage      conversation.Message
	savedResult       *conversation.Conversation
	existsValue       bool
	existsCalled      bool
	saveCalled        bool
	findByIDCalled    bool
	findByConsumerIDResult []conversation.Conversation
	findByProviderIDResult []conversation.Conversation
	foundResult       *conversation.Conversation
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

func (r *conversationRepositoryMock) FindByConsumerID(ctx context.Context, consumerID int) ([]conversation.Conversation, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.findByConsumerIDResult, nil
}

func (r *conversationRepositoryMock) FindByProviderID(ctx context.Context, providerID int) ([]conversation.Conversation, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.findByProviderIDResult, nil
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

type providerFinderMock struct {
	providers []provider.Provider
	err       error
}

func (m *providerFinderMock) FindByIDs(ctx context.Context, ids []int) ([]provider.Provider, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.providers, nil
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

type consumerFinderMock struct {
	consumers []consumer.Consumer
	err       error
}

func (m *consumerFinderMock) FindByIDs(ctx context.Context, ids []int) ([]consumer.Consumer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.consumers, nil
}

type messageFinderMock struct {
	messages map[int]conversation.Message
	err      error
}

func (m *messageFinderMock) FindLastMessagesByConversationIDs(ctx context.Context, conversationIDs []int) (map[int]conversation.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.messages, nil
}

func TestStartWorkRequestCreatesPendingConversationWithInitialMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

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
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|provider", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanStartWorkRequest)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsNonExistingProvider(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: false}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrProviderDoesNotExist)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsMissingProviderID(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 0, "Hola")

	assert.ErrorIs(t, err, conversation.ErrProviderRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsEmptyMessage(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsDuplicateConversation(t *testing.T) {
	repo := &conversationRepositoryMock{existsValue: true}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrAlreadyExists)
	assert.Nil(t, createdConversation)
	assert.True(t, repo.existsCalled)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsWhenProviderExistenceCheckerFails(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{err: errors.New("database error")}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.Error(t, err)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsWhenConversationExistenceCheckFails(t *testing.T) {
	repo := &conversationRepositoryMock{err: errors.New("database error")}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.Error(t, err)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.saveCalled)
}

func TestStartWorkRequestRejectsWhenMessageCreationFails(t *testing.T) {
	repo := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	providerIDFinder := &providerIDFinderMock{}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, repo.saveCalled)
}

func TestGetByIDReturnsConversationForParticipantConsumer(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 1, foundConversation.ID)
	assert.Equal(t, 10, foundConversation.ConsumerID)
	assert.Len(t, foundConversation.Messages, 1)
}

func TestGetByIDReturnsConversationForParticipantProvider(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|provider", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
	assert.Equal(t, 20, foundConversation.ProviderID)
}

func TestGetByIDRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	repo := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 888}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|other", 1)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestGetByIDReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	repo := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
	assert.True(t, repo.findByIDCalled)
}

func TestListReturnsConversationSummariesForConsumer(t *testing.T) {
	now := time.Now()
	repo := &conversationRepositoryMock{
		findByConsumerIDResult: []conversation.Conversation{
			{ID: 1, ConsumerID: 10, ProviderID: 20, Status: conversation.StatusPending, UpdatedOn: now},
		},
	}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerFinder := &providerFinderMock{
		providers: []provider.Provider{
			{
				ID: 20,
				User:     &user.User{Name: "Juan", Surname: "Gómez"},
				Category: &category.Category{Name: "Plomería"},
			},
		},
	}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{
		messages: map[int]conversation.Message{
			1: {ID: 1, ConversationID: 1, SenderRole: conversation.SenderConsumer, Content: "Hola", CreatedOn: now},
		},
	}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

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
	repo := &conversationRepositoryMock{
		findByProviderIDResult: []conversation.Conversation{
			{ID: 1, ConsumerID: 10, ProviderID: 20, Status: conversation.StatusPending, UpdatedOn: now},
		},
	}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	consumerFinder := &consumerFinderMock{
		consumers: []consumer.Consumer{
			{
				ID:   10,
				User: &user.User{Name: "Ana", Surname: "Pérez"},
			},
		},
	}
	messageFinder := &messageFinderMock{
		messages: map[int]conversation.Message{
			1: {ID: 1, ConversationID: 1, SenderRole: conversation.SenderConsumer, Content: "Hola", CreatedOn: now},
		},
	}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

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
	providerFinder := &providerFinderMock{}
	providerExistenceChecker := &providerExistenceCheckerMock{}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	consumerFinder := &consumerFinderMock{}
	messageFinder := &messageFinderMock{}

	service := conversation.NewService(repo, consumerIDFinder, providerFinder, providerExistenceChecker, providerIDFinder, consumerFinder, messageFinder)

	summaries, err := service.List(context.Background(), "auth0|unknown")

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, summaries)
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