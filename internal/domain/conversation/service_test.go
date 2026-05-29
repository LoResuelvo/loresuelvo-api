package conversation_test

import (
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
	return &conversationToSave, nil
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

func TestStartWorkRequestCreatesPendingConversationWithInitialMessage(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{consumerID: 10}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker)

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
}

func TestStartWorkRequestRejectsNonConsumerUser(t *testing.T) {
	conversationRepository := &conversationRepositoryMock{}
	consumerIDFinder := &consumerIDFinderMock{err: errors.New("consumer not found")}
	providerExistenceChecker := &providerExistenceCheckerMock{exists: true}
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker)

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
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker)

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
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker)

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
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker)

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
	service := conversation.NewService(conversationRepository, consumerIDFinder, providerExistenceChecker)

	createdConversation, err := service.StartWorkRequest("auth0|consumer", 20, "Hola")

	assert.ErrorIs(t, err, conversation.ErrAlreadyExists)
	assert.Nil(t, createdConversation)
	assert.True(t, conversationRepository.existsCalled)
	assert.False(t, conversationRepository.saveCalled)
}
