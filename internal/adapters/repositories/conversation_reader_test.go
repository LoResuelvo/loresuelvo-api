package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conversationReaderFixture struct {
	reader                 *repositories.ConversationReader
	conversationRepository *repositories.ConversationRepository
	consumerID             int
	providerID             int
	savedConversation      *conversation.Conversation
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
		reader:                 repositories.NewConversationReader(testContext.database),
		conversationRepository: testContext.conversationRepository,
		consumerID:             consumerID,
		providerID:             providerID,
		savedConversation:      savedConversation,
		initialMessage:         messageToSave,
	}
}

func TestConversationReaderFindsConsumerSummaries(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	summaries, err := fixture.reader.FindSummariesByConsumerID(context.Background(), fixture.consumerID)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, fixture.savedConversation.ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, fixture.providerID, summaries[0].Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, summaries[0].Counterpart.Role)
	assert.Equal(t, "Juan", summaries[0].Counterpart.Name)
	assert.Equal(t, "Gómez", summaries[0].Counterpart.Surname)
	assert.Equal(t, "Plomería", summaries[0].Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, summaries[0].LastMessage.Content)
	assert.NotZero(t, summaries[0].UpdatedOn)
}

func TestConversationReaderFindsProviderSummaries(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	summaries, err := fixture.reader.FindSummariesByProviderID(context.Background(), fixture.providerID)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, fixture.savedConversation.ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, fixture.consumerID, summaries[0].Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].Counterpart.Role)
	assert.Equal(t, "Ana", summaries[0].Counterpart.Name)
	assert.Equal(t, "Pérez", summaries[0].Counterpart.Surname)
	assert.Empty(t, summaries[0].Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, summaries[0].LastMessage.Content)
	assert.NotZero(t, summaries[0].UpdatedOn)
}

func TestConversationReaderReturnsEmptySummaryLists(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database)

	consumerSummaries, err := reader.FindSummariesByConsumerID(context.Background(), 999)
	require.NoError(t, err)
	assert.Empty(t, consumerSummaries)

	providerSummaries, err := reader.FindSummariesByProviderID(context.Background(), 999)
	require.NoError(t, err)
	assert.Empty(t, providerSummaries)
}

func TestConversationReaderFindsDetailForConsumer(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	detail, err := fixture.reader.FindDetailByIDForConsumer(context.Background(), fixture.savedConversation.ID)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, fixture.savedConversation.ID, detail.ID)
	assert.Equal(t, conversation.StatusPending, detail.Status)
	assert.Equal(t, fixture.providerID, detail.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, detail.Counterpart.Role)
	assert.Equal(t, "Juan", detail.Counterpart.Name)
	assert.Equal(t, "Gómez", detail.Counterpart.Surname)
	assert.Equal(t, "Plomería", detail.Counterpart.CategoryName)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, detail.Messages[0].SenderRole)
	assert.Equal(t, fixture.initialMessage.Content, detail.Messages[0].Content)
	assert.NotZero(t, detail.Messages[0].CreatedOn)
	assert.NotZero(t, detail.UpdatedOn)
}

func TestConversationReaderFindsDetailForProvider(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)

	detail, err := fixture.reader.FindDetailByIDForProvider(context.Background(), fixture.savedConversation.ID)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, fixture.savedConversation.ID, detail.ID)
	assert.Equal(t, conversation.StatusPending, detail.Status)
	assert.Equal(t, fixture.consumerID, detail.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, detail.Counterpart.Role)
	assert.Equal(t, "Ana", detail.Counterpart.Name)
	assert.Equal(t, "Pérez", detail.Counterpart.Surname)
	assert.Empty(t, detail.Counterpart.CategoryName)
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
	sentMessage, err := fixture.conversationRepository.AddMessage(context.Background(), fixture.savedConversation.ID, *providerMessage)
	require.NoError(t, err)

	consumerSummaries, err := fixture.reader.FindSummariesByConsumerID(context.Background(), fixture.consumerID)
	require.NoError(t, err)
	require.Len(t, consumerSummaries, 1)
	require.NotNil(t, consumerSummaries[0].LastMessage)
	assert.Equal(t, sentMessage.ID, consumerSummaries[0].LastMessage.ID)
	assert.Equal(t, conversation.SenderProvider, consumerSummaries[0].LastMessage.SenderRole)
	assert.Equal(t, sentMessage.Content, consumerSummaries[0].LastMessage.Content)
	assert.Equal(t, sentMessage.CreatedOn, consumerSummaries[0].UpdatedOn)

	detail, err := fixture.reader.FindDetailByIDForConsumer(context.Background(), fixture.savedConversation.ID)
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
	reader := repositories.NewConversationReader(testContext.database)

	detail, err := reader.FindDetailByIDForConsumer(context.Background(), 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, detail)
}
