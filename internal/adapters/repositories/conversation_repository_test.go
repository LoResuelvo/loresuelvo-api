package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conversationRepositoryTestContext struct {
	database               *sql.DB
	consumerRepository     *repositories.ConsumerRepository
	providerRepository     *repositories.ProviderRepository
	categoryRepository     *repositories.CategoryRepository
	conversationRepository *repositories.ConversationRepository
	messageRepository      *repositories.MessageRepository
}

func cleanConversationRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM messages")
	require.NoError(t, err, "could not clean messages")

	_, err = database.Exec("DELETE FROM conversations")
	require.NoError(t, err, "could not clean conversations")

	_, err = database.Exec("DELETE FROM providers")
	require.NoError(t, err, "could not clean providers")

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM files")
	require.NoError(t, err, "could not clean files")

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
		database:               database,
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

	providerToSave := validProviderWithData(t, testContext.categoryRepository, testContext.database, "auth0|conversation-provider", "conversation.provider@example.com", "Juan", "Gómez", "Plomería")
	_, err := testContext.providerRepository.Save(*providerToSave)
	require.NoError(t, err)

	providerID, err := testContext.providerRepository.FindIDByEmail(providerToSave.Email())
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

	return conversationToSave, *messageToSave
}

func TestConversationRepositoryCanSaveConversationWithMessage(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)

	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)

	require.NoError(t, err)
	require.NotNil(t, savedConversation)
	savedWorkConversation := savedConversation.(*conversation.WorkConversation)
	assert.NotZero(t, savedConversation.Base().ID)
	assert.Equal(t, consumerID, savedWorkConversation.ConsumerID)
	assert.Equal(t, providerID, savedWorkConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, savedConversation.Base().Status)
	require.Len(t, savedConversation.Messages(), 1)
	assert.NotZero(t, savedConversation.Messages()[0].ID)
	assert.Equal(t, savedConversation.Base().ID, savedConversation.Messages()[0].ConversationID)
	assert.Equal(t, conversation.SenderConsumer, savedConversation.Messages()[0].SenderRole)
	assert.Equal(t, messageToSave.Content, savedConversation.Messages()[0].Content)
	assert.NotZero(t, savedConversation.Messages()[0].CreatedOn)
	assert.NotZero(t, savedConversation.Base().UpdatedOn)

	messageExists, err := testContext.messageRepository.ExistsInConversation(savedConversation.Base().ID, messageToSave.Content)
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
	assert.Equal(t, savedConversation.Base().ID, foundConversation.Base().ID)
	assert.Equal(t, conversation.StatusPending, foundConversation.Base().Status)
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

	exists, err := testContext.messageRepository.ExistsInConversation(savedConversation.Base().ID, "otro mensaje")

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
	messageExists, err := testContext.messageRepository.ExistsInConversation(savedConversation.Base().ID, messageToSave.Content)
	require.NoError(t, err)
	assert.False(t, messageExists)

	conversationExists, err := testContext.conversationRepository.ExistsBetween(consumerID, providerID)
	require.NoError(t, err)
	assert.True(t, conversationExists)
}

func TestConversationRepositoryCanFindByID(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.Base().ID)

	require.NoError(t, err)
	require.NotNil(t, foundConversation)
	foundWorkConversation := foundConversation.(*conversation.WorkConversation)
	assert.Equal(t, savedConversation.Base().ID, foundConversation.Base().ID)
	assert.Equal(t, consumerID, foundWorkConversation.ConsumerID)
	assert.Equal(t, providerID, foundWorkConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, foundConversation.Base().Status)
	require.Len(t, foundConversation.Messages(), 1)
	assert.Equal(t, messageToSave.Content, foundConversation.Messages()[0].Content)
}

func TestConversationRepositoryFindByIDReturnsNotFoundIfConversationDoesNotExist(t *testing.T) {
	testContext := newConversationRepositoryTest(t)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), 999999999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, foundConversation)
}

func TestConversationRepositoryCanSaveStatus(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)
	require.NoError(t, savedConversation.Activate())

	err = testContext.conversationRepository.SaveStatus(context.Background(), savedConversation)

	require.NoError(t, err)
	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.Base().ID)
	require.NoError(t, err)
	assert.Equal(t, conversation.StatusActive, foundConversation.Base().Status)
}

func TestConversationRepositorySaveStatusReturnsNotFoundIfConversationDoesNotExist(t *testing.T) {
	testContext := newConversationRepositoryTest(t)

	err := testContext.conversationRepository.SaveStatus(context.Background(), &conversation.WorkConversation{
		BaseConversation: &conversation.BaseConversation{
			ID:     999999999,
			Type:   conversation.TypeWork,
			Status: conversation.StatusActive,
		},
	})

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
}

func TestConversationRepositoryCanAddMessageToConversation(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	providerMessage, err := conversation.NewProviderMessage("Sí, puedo pasar el jueves a las 10")
	require.NoError(t, err)

	savedMessage, err := testContext.conversationRepository.AddMessage(context.Background(), savedConversation.Base().ID, *providerMessage)

	require.NoError(t, err)
	require.NotNil(t, savedMessage)
	assert.NotZero(t, savedMessage.ID)
	assert.Equal(t, savedConversation.Base().ID, savedMessage.ConversationID)
	assert.Equal(t, conversation.SenderProvider, savedMessage.SenderRole)
	assert.Equal(t, "Sí, puedo pasar el jueves a las 10", savedMessage.Content)
	assert.NotZero(t, savedMessage.CreatedOn)

	messageExists, err := testContext.messageRepository.ExistsInConversation(savedConversation.Base().ID, savedMessage.Content)
	require.NoError(t, err)
	assert.True(t, messageExists)

	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.Base().ID)
	require.NoError(t, err)
	assert.Equal(t, savedMessage.CreatedOn, foundConversation.Base().UpdatedOn)
}

func TestConversationRepositoryAddMessageReturnsNotFoundForMissingConversation(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	providerMessage, err := conversation.NewProviderMessage("Sí, puedo pasar el jueves a las 10")
	require.NoError(t, err)

	savedMessage, err := testContext.conversationRepository.AddMessage(context.Background(), 999999999, *providerMessage)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, savedMessage)
}

func savedChatbotConversationForRepository(t *testing.T, testContext conversationRepositoryTestContext) conversation.Conversation {
	t.Helper()

	consumerID := savedConsumerIDForConversation(t, testContext)
	chatbotConversation, err := conversation.NewChatbotConversation(consumerID, "Pérdida de agua en la cocina")
	require.NoError(t, err)

	consumerMessage, err := conversation.NewConsumerMessage("Tengo una pérdida debajo de la pileta.")
	require.NoError(t, err)
	chatbotMessage, err := conversation.NewChatbotMessage("Revisá si el agua aparece cuando usás la bacha.")
	require.NoError(t, err)
	chatbotConversation.AddMessage(*consumerMessage)
	chatbotConversation.AddMessage(*chatbotMessage)

	savedConversation, err := testContext.conversationRepository.SaveConversation(context.Background(), chatbotConversation)
	require.NoError(t, err)
	return savedConversation
}

func TestConversationRepositoryCanUpdateChatbotContext(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	savedConversation := savedChatbotConversationForRepository(t, testContext)

	savedChatbotConversation := savedConversation.(*conversation.ChatBotConversation)
	require.NoError(t, savedChatbotConversation.UpdateContext(conversation.ChatbotConversationContext{Summary: "La consumidora reportó una pérdida bajo la pileta.", LastSummarizedMessageID: 2}))
	updatedConversation, err := testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)

	require.NoError(t, err)
	require.NotNil(t, updatedConversation)
	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.Base().ID)
	require.NoError(t, err)
	foundChatbotConversation := foundConversation.(*conversation.ChatBotConversation)
	assert.Equal(t, "La consumidora reportó una pérdida bajo la pileta.", foundChatbotConversation.Context.Summary)
	assert.Equal(t, 2, foundChatbotConversation.Context.LastSummarizedMessageID)
	require.Len(t, foundChatbotConversation.Messages(), 2)
	assert.Equal(t, conversation.SenderConsumer, foundChatbotConversation.Messages()[0].SenderRole)
	assert.Equal(t, conversation.SenderChatbot, foundChatbotConversation.Messages()[1].SenderRole)
}

func TestConversationRepositoryCanUpdateConversationWithChatbotTurn(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	savedConversation := savedChatbotConversationForRepository(t, testContext)

	consumerMessage, err := conversation.NewConsumerMessage("El agua sale cerca de la rosca del sifón.")
	require.NoError(t, err)
	chatbotMessage, err := conversation.NewChatbotMessage("Entonces revisá y ajustá la rosca del sifón.")
	require.NoError(t, err)

	savedChatbotConversation := savedConversation.(*conversation.ChatBotConversation)
	require.NoError(t, savedChatbotConversation.UpdateContext(conversation.ChatbotConversationContext{Summary: "Resumen actualizado"}))
	require.NoError(t, savedChatbotConversation.AddTurn(*consumerMessage, *chatbotMessage))
	savedChatbotConversation.FinishProcessing()
	updatedConversation, err := testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)

	require.NoError(t, err)
	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), updatedConversation.Base().ID)
	require.NoError(t, err)
	foundChatbotConversation := foundConversation.(*conversation.ChatBotConversation)
	assert.Equal(t, "Resumen actualizado", foundChatbotConversation.Context.Summary)
	require.Len(t, foundChatbotConversation.Messages(), 4)
	assert.Equal(t, conversation.SenderConsumer, foundChatbotConversation.Messages()[2].SenderRole)
	assert.Equal(t, conversation.SenderChatbot, foundChatbotConversation.Messages()[3].SenderRole)
	assert.Zero(t, foundChatbotConversation.Context.LastSummarizedMessageID)
}

func TestConversationRepositoryPreventsConcurrentChatbotProcessing(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	savedConversation := savedChatbotConversationForRepository(t, testContext)
	concurrentSnapshot, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.Base().ID)
	require.NoError(t, err)
	concurrentConversation := concurrentSnapshot.(*conversation.ChatBotConversation)
	savedChatbotConversation := savedConversation.(*conversation.ChatBotConversation)

	require.NoError(t, savedChatbotConversation.StartProcessing(time.Now().UTC()))
	_, err = testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)
	require.NoError(t, err)

	require.NoError(t, concurrentConversation.StartProcessing(time.Now().UTC()))
	_, err = testContext.conversationRepository.UpdateConversation(context.Background(), concurrentConversation)
	assert.ErrorIs(t, err, conversation.ErrChatbotConversationAlreadyProcessing)

	savedChatbotConversation.FinishProcessing()
	_, err = testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)
	require.NoError(t, err)
	require.NoError(t, savedChatbotConversation.StartProcessing(time.Now().UTC()))
	_, err = testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)
	assert.NoError(t, err)
}

func TestConversationRepositoryAllowsContextUpdateWhileSameChatbotProcessingOwnsConversation(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	savedConversation := savedChatbotConversationForRepository(t, testContext)
	savedChatbotConversation := savedConversation.(*conversation.ChatBotConversation)
	startedOn := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, savedChatbotConversation.StartProcessing(startedOn))
	_, err := testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)
	require.NoError(t, err)
	require.NoError(t, savedChatbotConversation.UpdateContext(conversation.ChatbotConversationContext{
		Summary:                 "Contexto actualizado durante el procesamiento",
		LastSummarizedMessageID: 2,
	}))

	_, err = testContext.conversationRepository.UpdateConversation(context.Background(), savedChatbotConversation)

	require.NoError(t, err)
	foundConversation, err := testContext.conversationRepository.FindByID(context.Background(), savedConversation.Base().ID)
	require.NoError(t, err)
	foundChatbotConversation := foundConversation.(*conversation.ChatBotConversation)
	assert.Equal(t, "Contexto actualizado durante el procesamiento", foundChatbotConversation.Context.Summary)
	assert.Equal(t, 2, foundChatbotConversation.Context.LastSummarizedMessageID)
	require.NotNil(t, foundChatbotConversation.ProcessingStartedAt())
	assert.Equal(t, startedOn, *foundChatbotConversation.ProcessingStartedAt())
}
