package conversation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
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
	foundResult       *conversation.Conversation
	err               error
}

type consumerIDFinderMock struct {
	consumerID int
	err        error
}

type providerExistenceCheckerMock struct {
	exists bool
	err    error
}

type providerIDFinderMock struct {
	providerID int
	err        error
}

func (repository *conversationRepositoryMock) ExistsBetween(consumerID, providerID int) (bool, error) {
	repository.existsCalled = true
	if repository.err != nil {
		return false, repository.err
	}
	return repository.existsValue, nil
}

func (repository *conversationRepositoryMock) SaveWithMessage(conversationToSave conversation.Conversation, message conversation.Message) (*conversation.Conversation, error) {
	repository.saveCalled = true
	repository.savedConversation = conversationToSave
	repository.savedMessage = message
	if repository.err != nil {
		return nil, repository.err
	}
	if repository.savedResult != nil {
		return repository.savedResult, nil
	}
	conversationToSave.ID = 1
	message.ConversationID = conversationToSave.ID
	message.ID = 1
	conversationToSave.Messages = []conversation.Message{message}
	return &conversationToSave, nil
}

func (repository *conversationRepositoryMock) FindByID(ctx context.Context, conversationID int) (*conversation.Conversation, error) {
	repository.findByIDCalled = true
	if repository.err != nil {
		return nil, repository.err
	}
	if repository.foundResult != nil {
		return repository.foundResult, nil
	}
	return nil, conversation.ErrConversationDoesNotExist
}

func (repository *consumerIDFinderMock) FindIDByAuthID(authID string) (int, error) {
	if repository.err != nil {
		return 0, repository.err
	}
	return repository.consumerID, nil
}

func (repository *providerExistenceCheckerMock) ExistsByID(id int) (bool, error) {
	if repository.err != nil {
		return false, repository.err
	}
	return repository.exists, nil
}

func (repository *providerIDFinderMock) FindIDByAuthID(authID string) (int, error) {
	if repository.err != nil {
		return 0, repository.err
	}
	return repository.providerID, nil
}

func TestStartWorkRequestCreatesPendingConversationWithInitialMessage(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker, &providerIDFinderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "  Hola, necesito un presupuesto  ")

	require.NoError(t, err)
	require.NotNil(t, createdConversation)
	assert.Equal(t, 1, createdConversation.ID)
	assert.True(t, conversationRepository.existsCalled)
	assert.True(t, conversationRepository.saveCalled)
	assert.Equal(t, 10, conversationRepository.savedConversation.ConsumerID)
	assert.Equal(t, 20, conversationRepository.savedConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, conversationRepository.savedConversation.Status)
	assert.Equal(t, conversation.SenderConsumer, conversationRepository.savedMessage.SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", conversationRepository.savedMessage.Content)
	require.Len(t, createdConversation.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, createdConversation.Messages[0].SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", createdConversation.Messages[0].Content)
}

func TestStartWorkRequestRejectsNonConsumerUser(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker, &providerIDFinderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|provider", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanStartWorkRequest)
	assert.Nil(t, createdConversation)
	assert.False(t, conversationRepository.existsCalled)
	assert.False(t, conversationRepository.saveCalled)
}

func TestStartWorkRequestRejectsNonExistingProvider(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: false}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker, &providerIDFinderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrProviderDoesNotExist)
	assert.Nil(t, createdConversation)
	assert.False(t, conversationRepository.existsCalled)
	assert.False(t, conversationRepository.saveCalled)
}

func TestStartWorkRequestRejectsMissingProviderID(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker, &providerIDFinderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 0, "Hola")

	assert.ErrorIs(t, err, conversation.ErrProviderRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, conversationRepository.existsCalled)
	assert.False(t, conversationRepository.saveCalled)
}

func TestStartWorkRequestRejectsEmptyMessage(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker, &providerIDFinderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, createdConversation)
	assert.False(t, conversationRepository.existsCalled)
	assert.False(t, conversationRepository.saveCalled)
}

func TestStartWorkRequestRejectsDuplicateConversation(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{existsValue: true}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker, &providerIDFinderMock{})

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrAlreadyExists)
	assert.Nil(t, createdConversation)
	assert.True(t, conversationRepository.existsCalled)
	assert.False(t, conversationRepository.saveCalled)
}

func TestGetByIDReturnsConversationForParticipantConsumer(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{err: errors.New("provider not found")}
	service := conversation.NewService(conversationRepository, consumerIDFinder, &providerExistenceCheckerMock{}, providerIDFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, conversationRepository.findByIDCalled)
	assert.Equal(t, 1, foundConversation.ID)
	assert.Equal(t, 10, foundConversation.ConsumerID)
	assert.Len(t, foundConversation.Messages, 1)
}

func TestGetByIDReturnsConversationForParticipantProvider(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	service := conversation.NewService(conversationRepository, consumerIDFinder, &providerExistenceCheckerMock{}, providerIDFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|provider", 1)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.True(t, conversationRepository.findByIDCalled)
	assert.Equal(t, 20, foundConversation.ProviderID)
}

func TestGetByIDRejectsAuthenticatedUserThatIsNotParticipant(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{foundResult: conversationFixture()}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 999}
	providerIDFinder := &providerIDFinderMock{providerID: 888}
	service := conversation.NewService(conversationRepository, consumerIDFinder, &providerExistenceCheckerMock{}, providerIDFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|other", 1)

	assert.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
	assert.Nil(t, foundConversation)
	assert.True(t, conversationRepository.findByIDCalled)
}

func TestGetByIDReturnsNotFoundWhenConversationDoesNotExist(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{err: conversation.ErrConversationDoesNotExist}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerIDFinder := &providerIDFinderMock{providerID: 20}
	service := conversation.NewService(conversationRepository, consumerIDFinder, &providerExistenceCheckerMock{}, providerIDFinder)

	foundConversation, err := service.GetByID(context.Background(), "auth0|consumer", 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
	assert.True(t, conversationRepository.findByIDCalled)
}

func conversationFixture() *conversation.Conversation {
	return &conversation.Conversation{
		ID:         1,
		ConsumerID: 10,
		ProviderID: 20,
		Status:     conversation.StatusPending,
		Messages: []conversation.Message{
			{
				ID:             1,
				ConversationID: 1,
				SenderRole:     conversation.SenderConsumer,
				Content:        "Hola, necesito un presupuesto",
			},
		},
	}
}
