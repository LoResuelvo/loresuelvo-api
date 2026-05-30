package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conversationRepositoryTestContext struct {
	consumerRepository     *repositories.ConsumerRepository
	providerRepository     *repositories.ProviderRepository
	categoryRepository     *repositories.CategoryRepository
	conversationRepository *repositories.ConversationRepository
	messageRepository      *repositories.MessageRepository
}

func cleanConversationRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM categories")
	require.NoError(t, err, "could not clean categories")
}

func newConversationRepositoryTest(t *testing.T) conversationRepositoryTestContext {
	t.Helper()

	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanConversationRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanConversationRepositoryTestDatabase(t, database)

	userRepository := repositories.NewUserRepository(database)
	messageRepository := repositories.NewMessageRepository(database)
	return conversationRepositoryTestContext{
		consumerRepository:     repositories.NewConsumerRepository(database, userRepository),
		providerRepository:     repositories.NewProviderRepository(database, userRepository),
		categoryRepository:     repositories.NewCategoryRepository(database),
		conversationRepository: repositories.NewConversationRepository(database, messageRepository),
		messageRepository:      messageRepository,
	}
}

func savedConsumerIDForConversation(t *testing.T, testContext conversationRepositoryTestContext) int {
	t.Helper()

	consumerToSave, err := consumer.NewConsumer("auth0|conversation-consumer", "conversation.consumer@example.com", "Ana", "Pérez")
	require.NoError(t, err)
	require.NoError(t, testContext.consumerRepository.Save(*consumerToSave))

	consumerID, err := testContext.consumerRepository.FindIDByEmail(consumerToSave.User.Email)
	require.NoError(t, err)
	return consumerID
}

func savedProviderIDForConversation(t *testing.T, testContext conversationRepositoryTestContext) int {
	t.Helper()

	providerToSave := validProviderWithData(t, testContext.categoryRepository, "auth0|conversation-provider", "conversation.provider@example.com", "Juan", "Gómez", "Plomería")
	require.NoError(t, testContext.providerRepository.Save(*providerToSave))

	providerID, err := testContext.providerRepository.FindIDByEmail(providerToSave.User.Email)
	require.NoError(t, err)
	return providerID
}

func savedConversationParticipants(t *testing.T, testContext conversationRepositoryTestContext) (int, int) {
	t.Helper()

	return savedConsumerIDForConversation(t, testContext), savedProviderIDForConversation(t, testContext)
}

func pendingConversationWithMessage(t *testing.T, consumerID, providerID int) (conversation.Conversation, conversation.Message) {
	t.Helper()

	conversationToSave, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)

	messageToSave, err := conversation.NewConsumerMessage("Hola, necesito un presupuesto")
	require.NoError(t, err)

	return *conversationToSave, *messageToSave
}

func TestConversationRepositoryCanSaveConversationWithMessage(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)

	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)

	require.NoError(t, err)
	require.NotNil(t, savedConversation)
	assert.NotZero(t, savedConversation.ID)
	assert.Equal(t, consumerID, savedConversation.ConsumerID)
	assert.Equal(t, providerID, savedConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, savedConversation.Status)
	require.Len(t, savedConversation.Messages, 1)
	assert.NotZero(t, savedConversation.Messages[0].ID)
	assert.Equal(t, savedConversation.ID, savedConversation.Messages[0].ConversationID)
	assert.Equal(t, conversation.SenderConsumer, savedConversation.Messages[0].SenderRole)
	assert.Equal(t, messageToSave.Content, savedConversation.Messages[0].Content)

	messageExists, err := testContext.messageRepository.ExistsInConversation(savedConversation.ID, messageToSave.Content)
	require.NoError(t, err)
	assert.True(t, messageExists, "initial message should be saved with the conversation")
}

func TestConversationRepositoryRejectsDuplicateConversation(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)

	_, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	duplicateConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)

	assert.ErrorIs(t, err, conversation.ErrAlreadyExists)
	assert.Nil(t, duplicateConversation)
}

func TestConversationRepositoryCanCheckExistenceBetweenParticipants(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)

	existsBeforeSave, err := testContext.conversationRepository.ExistsBetween(consumerID, providerID)
	require.NoError(t, err)
	assert.False(t, existsBeforeSave)

	_, err = testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	existsAfterSave, err := testContext.conversationRepository.ExistsBetween(consumerID, providerID)
	require.NoError(t, err)
	assert.True(t, existsAfterSave)
}

func TestConversationRepositoryCanFindConversationBetweenParticipants(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	foundConversation, err := testContext.conversationRepository.FindBetween(consumerID, providerID)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.Equal(t, savedConversation.ID, foundConversation.ID)
	assert.Equal(t, conversation.StatusPending, foundConversation.Status)
}

func TestConversationRepositoryCanFindByConsumerID(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	conversations, err := testContext.conversationRepository.FindByConsumerID(consumerID)

	require.NoError(t, err)
	require.Len(t, conversations, 1)
	assert.Equal(t, savedConversation.ID, conversations[0].ID)
}

func TestConversationRepositoryCanDeleteBetweenParticipants(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	_, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	err = testContext.conversationRepository.DeleteBetween(consumerID, providerID)

	require.NoError(t, err)
	exists, err := testContext.conversationRepository.ExistsBetween(consumerID, providerID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestConversationRepositoryCanDeleteAllConversations(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	_, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	err = testContext.conversationRepository.DeleteAll()

	require.NoError(t, err)
	exists, err := testContext.conversationRepository.ExistsBetween(consumerID, providerID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMessageRepositoryReportsMissingMessageInConversation(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	exists, err := testContext.messageRepository.ExistsInConversation(savedConversation.ID, "otro mensaje")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMessageRepositoryCanDeleteAllMessages(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	err = testContext.messageRepository.DeleteAll()

	require.NoError(t, err)
	messageExists, err := testContext.messageRepository.ExistsInConversation(savedConversation.ID, messageToSave.Content)
	require.NoError(t, err)
	assert.False(t, messageExists)

	conversationExists, err := testContext.conversationRepository.ExistsBetween(consumerID, providerID)
	require.NoError(t, err)
	assert.True(t, conversationExists)
}

func TestConversationRepositoryCanFindByIDWithMessages(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.ID)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	assert.Equal(t, savedConversation.ID, foundConversation.ID)
	assert.Equal(t, consumerID, foundConversation.ConsumerID)
	assert.Equal(t, providerID, foundConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, foundConversation.Status)
	require.Len(t, foundConversation.Messages, 1)
	assert.Equal(t, savedConversation.ID, foundConversation.Messages[0].ConversationID)
	assert.Equal(t, conversation.SenderConsumer, foundConversation.Messages[0].SenderRole)
	assert.Equal(t, messageToSave.Content, foundConversation.Messages[0].Content)
}

func TestConversationRepositoryFindByIDReturnsNotFoundIfConversationDoesNotExist(t *testing.T) {
	testContext := newConversationRepositoryTest(t)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), 999999999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
}

func TestMessageRepositoryCanFindMessagesByConversationID(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	messages, err := testContext.messageRepository.FindByConversationID(context.Background(), savedConversation.ID)

	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.NotZero(t, messages[0].ID)
	assert.Equal(t, savedConversation.ID, messages[0].ConversationID)
	assert.Equal(t, conversation.SenderConsumer, messages[0].SenderRole)
	assert.Equal(t, messageToSave.Content, messages[0].Content)
}
