package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conversationReaderFixture struct {
	reader                 *repositories.ConversationReader
	conversationRepository *repositories.ConversationRepository
	consumerID             int
	providerID             int
	savedConversation      conversation.Conversation
	initialMessage         conversation.Message
}

func newSavedConversationReaderFixture(t *testing.T) conversationReaderFixture {
	t.Helper()

	testContext := newConversationRepositoryTest(t)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	return conversationReaderFixture{
		reader:                 repositories.NewConversationReader(testContext.database, repositories.NewMessageImageRepository(testContext.database)),
		conversationRepository: testContext.conversationRepository,
		consumerID:             consumerID,
		providerID:             providerID,
		savedConversation:      savedConversation,
		initialMessage:         messageToSave,
	}
}

func TestConversationReaderFindsConsumerSummaries(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	summaries, err := fixture.reader.FindSummariesByParticipantIDRoleAndType(context.Background(), fixture.consumerID, conversation.SenderConsumer, conversation.TypeWork)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, fixture.savedConversation.Base().ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, fixture.providerID, summaries[0].Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, summaries[0].Work.Counterpart.Role)
	assert.Equal(t, "Juan", summaries[0].Work.Counterpart.Name)
	assert.Equal(t, "Gómez", summaries[0].Work.Counterpart.Surname)
	assert.Equal(t, "Plomería", summaries[0].Work.Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, summaries[0].LastMessage.Content)
	assert.NotZero(t, summaries[0].UpdatedOn)
}

func TestConversationReaderFindsProviderSummaries(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	summaries, err := fixture.reader.FindSummariesByParticipantIDRoleAndType(context.Background(), fixture.providerID, conversation.SenderProvider, conversation.TypeWork)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, fixture.savedConversation.Base().ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, fixture.consumerID, summaries[0].Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].Work.Counterpart.Role)
	assert.Equal(t, "Ana", summaries[0].Work.Counterpart.Name)
	assert.Equal(t, "Pérez", summaries[0].Work.Counterpart.Surname)
	assert.Empty(t, summaries[0].Work.Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, summaries[0].LastMessage.Content)
	assert.NotZero(t, summaries[0].UpdatedOn)
}

func TestConversationReaderReturnsEmptySummaryLists(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database, repositories.NewMessageImageRepository(testContext.database))

	consumerSummaries, err := reader.FindSummariesByParticipantIDRoleAndType(context.Background(), 999, conversation.SenderConsumer, conversation.TypeWork)
	require.NoError(t, err)
	assert.Empty(t, consumerSummaries)

	providerSummaries, err := reader.FindSummariesByParticipantIDRoleAndType(context.Background(), 999, conversation.SenderProvider, conversation.TypeWork)
	require.NoError(t, err)
	assert.Empty(t, providerSummaries)
}

func TestConversationReaderFindsDetailForConsumer(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	detail, err := fixture.reader.FindDetailByIDRoleAndType(context.Background(), fixture.savedConversation.Base().ID, conversation.SenderConsumer, conversation.TypeWork)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, fixture.savedConversation.Base().ID, detail.ID)
	assert.Equal(t, conversation.StatusPending, detail.Status)
	assert.Equal(t, fixture.providerID, detail.Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, detail.Work.Counterpart.Role)
	assert.Equal(t, "Juan", detail.Work.Counterpart.Name)
	assert.Equal(t, "Gómez", detail.Work.Counterpart.Surname)
	assert.Equal(t, "Plomería", detail.Work.Counterpart.CategoryName)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, detail.Messages[0].SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, detail.Messages[0].Content)
	assert.NotZero(t, detail.Messages[0].CreatedOn)
	assert.NotZero(t, detail.UpdatedOn)
}

func TestConversationReaderFindsDetailForProvider(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	detail, err := fixture.reader.FindDetailByIDRoleAndType(context.Background(), fixture.savedConversation.Base().ID, conversation.SenderProvider, conversation.TypeWork)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, fixture.savedConversation.Base().ID, detail.ID)
	assert.Equal(t, conversation.StatusPending, detail.Status)
	assert.Equal(t, fixture.consumerID, detail.Work.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, detail.Work.Counterpart.Role)
	assert.Equal(t, "Ana", detail.Work.Counterpart.Name)
	assert.Equal(t, "Pérez", detail.Work.Counterpart.Surname)
	assert.Empty(t, detail.Work.Counterpart.CategoryName)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, detail.Messages[0].SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, detail.Messages[0].Content)
	assert.NotZero(t, detail.Messages[0].CreatedOn)
	assert.NotZero(t, detail.UpdatedOn)
}

func TestConversationReaderReflectsSentMessageAsLatestAndDetailLastMessage(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	providerMessage, err := conversation.NewProviderMessage("Sí, puedo pasar el jueves a las 10")
	require.NoError(t, err)
	sentMessage, err := fixture.conversationRepository.AddMessage(context.Background(), fixture.savedConversation.Base().ID, *providerMessage)
	require.NoError(t, err)

	consumerSummaries, err := fixture.reader.FindSummariesByParticipantIDRoleAndType(context.Background(), fixture.consumerID, conversation.SenderConsumer, conversation.TypeWork)
	require.NoError(t, err)
	require.Len(t, consumerSummaries, 1)
	require.NotNil(t, consumerSummaries[0].LastMessage)
	assert.Equal(t, sentMessage.ID, consumerSummaries[0].LastMessage.ID)
	assert.Equal(t, conversation.SenderProvider, consumerSummaries[0].LastMessage.SenderRole)
	assert.Equal(t, sentMessage.Content, consumerSummaries[0].LastMessage.Content)
	assert.Equal(t, sentMessage.CreatedOn, consumerSummaries[0].UpdatedOn)

	detail, err := fixture.reader.FindDetailByIDRoleAndType(context.Background(), fixture.savedConversation.Base().ID, conversation.SenderConsumer, conversation.TypeWork)
	require.NoError(t, err)
	require.Len(t, detail.Messages, 2)
	assert.Equal(t, fixture.initialMessage.Content, detail.Messages[0].Content)
	assert.Equal(t, sentMessage.ID, detail.Messages[1].ID)
	assert.Equal(t, conversation.SenderProvider, detail.Messages[1].SenderRole)
	assert.Equal(t, sentMessage.Content, detail.Messages[1].Content)
	assert.Equal(t, sentMessage.CreatedOn, detail.UpdatedOn)
}

func TestConversationReaderReturnsNotFoundForMissingConversationDetail(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database, repositories.NewMessageImageRepository(testContext.database))

	detail, err := reader.FindDetailByIDRoleAndType(context.Background(), 999, conversation.SenderConsumer, conversation.TypeWork)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, detail)
}

func TestConversationReaderFindsChatbotSummariesByConsumerIDAndType(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database, repositories.NewMessageImageRepository(testContext.database))
	consumerID, providerID := savedConversationParticipants(t, testContext)
	otherConsumerID := savedConsumerIDForConversationWithData(t, testContext, "auth0|other-chatbot-consumer", "other.chatbot.consumer@example.com", "Diego", "Sosa")

	chatbotConversation, err := conversation.NewChatbotConversation(consumerID, "Pérdida de agua en la cocina")
	require.NoError(t, err)
	consumerMessage, err := conversation.NewConsumerMessage("Tengo una pérdida debajo de la pileta.")
	require.NoError(t, err)
	chatbotMessage, err := conversation.NewChatbotMessage("Revisá el sifón y las conexiones flexibles.")
	require.NoError(t, err)
	chatbotConversation.AddMessage(*consumerMessage)
	chatbotConversation.AddMessage(*chatbotMessage)
	savedChatbotConversation, err := testContext.conversationRepository.SaveConversation(context.Background(), chatbotConversation)
	require.NoError(t, err)

	otherChatbotConversation, err := conversation.NewChatbotConversation(otherConsumerID, "Problema eléctrico")
	require.NoError(t, err)
	otherChatbotConversation.AddMessage(*consumerMessage)
	_, err = testContext.conversationRepository.SaveConversation(context.Background(), otherChatbotConversation)
	require.NoError(t, err)

	workConversation, workMessage := pendingConversationWithMessage(t, consumerID, providerID)
	_, err = testContext.conversationRepository.SaveWithMessage(workConversation, workMessage)
	require.NoError(t, err)

	summaries, err := reader.FindSummariesByParticipantIDRoleAndType(context.Background(), consumerID, conversation.SenderConsumer, conversation.TypeChatbot)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, savedChatbotConversation.Base().ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusActive, summaries[0].Status)
	assert.Equal(t, "Pérdida de agua en la cocina", summaries[0].Chatbot.Title)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderChatbot, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, chatbotMessage.Content, summaries[0].LastMessage.Content)
	assert.NotZero(t, summaries[0].UpdatedOn)
}

func TestConversationReaderFindsChatbotDetailByIDRoleAndType(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database, repositories.NewMessageImageRepository(testContext.database))
	consumerID := savedConsumerIDForConversation(t, testContext)
	plumbingCategory, err := category.New("Plomería")
	require.NoError(t, err)
	savedCategory, err := testContext.categoryRepository.Save(*plumbingCategory)
	require.NoError(t, err)

	chatbotConversation, err := conversation.NewChatbotConversation(consumerID, "Pérdida de agua en la cocina")
	require.NoError(t, err)
	typedChatbotConversation := chatbotConversation.(*conversation.ChatBotConversation)
	typedChatbotConversation.ApplyResponse(conversation.ChatbotResponse{
		Status:             conversation.ChatbotResponseAnswered,
		DiagnosisCompleted: true,
	}, &savedCategory.ID)
	consumerMessage, err := conversation.NewConsumerMessage("Tengo una pérdida debajo de la pileta.")
	require.NoError(t, err)
	chatbotMessage, err := conversation.NewChatbotMessage("Revisá el sifón y las conexiones flexibles.")
	require.NoError(t, err)
	typedChatbotConversation.AddMessage(*consumerMessage)
	typedChatbotConversation.AddMessage(*chatbotMessage)
	savedConversation, err := testContext.conversationRepository.SaveConversation(context.Background(), typedChatbotConversation)
	require.NoError(t, err)

	detail, err := reader.FindDetailByIDRoleAndType(context.Background(), savedConversation.Base().ID, conversation.SenderConsumer, conversation.TypeChatbot)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, savedConversation.Base().ID, detail.ID)
	assert.Equal(t, conversation.TypeChatbot, detail.Type)
	assert.Equal(t, conversation.StatusActive, detail.Status)
	require.NotNil(t, detail.Chatbot)
	assert.Equal(t, "Pérdida de agua en la cocina", detail.Chatbot.Title)
	assert.Equal(t, string(conversation.ChatbotResponseAnswered), detail.Chatbot.ResponseStatus)
	assert.True(t, detail.Chatbot.DiagnosisCompleted)
	require.NotNil(t, detail.Chatbot.RecommendedCategory)
	assert.Equal(t, savedCategory.ID, detail.Chatbot.RecommendedCategory.ID)
	assert.Equal(t, "Plomería", detail.Chatbot.RecommendedCategory.Name)
	require.Len(t, detail.Messages, 2)
	assert.Equal(t, conversation.SenderConsumer, detail.Messages[0].SenderRole)
	assert.Equal(t, consumerMessage.Content, detail.Messages[0].Content)
	assert.Equal(t, conversation.SenderChatbot, detail.Messages[1].SenderRole)
	assert.Equal(t, chatbotMessage.Content, detail.Messages[1].Content)
	assert.NotZero(t, detail.UpdatedOn)
}

func TestConversationReaderReturnsEmptyChatbotSummariesForDifferentType(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database, repositories.NewMessageImageRepository(testContext.database))
	consumerID := savedConsumerIDForConversation(t, testContext)
	chatbotConversation, err := conversation.NewChatbotConversation(consumerID, "Pérdida de agua en la cocina")
	require.NoError(t, err)
	consumerMessage, err := conversation.NewConsumerMessage("Tengo una pérdida debajo de la pileta.")
	require.NoError(t, err)
	chatbotConversation.AddMessage(*consumerMessage)
	_, err = testContext.conversationRepository.SaveConversation(context.Background(), chatbotConversation)
	require.NoError(t, err)

	summaries, err := reader.FindSummariesByParticipantIDRoleAndType(context.Background(), consumerID, conversation.SenderConsumer, conversation.TypeWork)

	require.NoError(t, err)
	assert.Empty(t, summaries)
}
