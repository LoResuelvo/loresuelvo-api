package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationReaderFindsConsumerSummaries(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	summaries, err := reader.FindSummariesByConsumerID(context.Background(), consumerID)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, savedConversation.ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, providerID, summaries[0].Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, summaries[0].Counterpart.Role)
	assert.Equal(t, "Juan", summaries[0].Counterpart.Name)
	assert.Equal(t, "Gómez", summaries[0].Counterpart.Surname)
	assert.Equal(t, "Plomería", summaries[0].Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, messageToSave.Content, summaries[0].LastMessage.Content)
	assert.NotZero(t, summaries[0].UpdatedOn)
}

func TestConversationReaderFindsProviderSummaries(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	summaries, err := reader.FindSummariesByProviderID(context.Background(), providerID)

	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, savedConversation.ID, summaries[0].ID)
	assert.Equal(t, conversation.StatusPending, summaries[0].Status)
	assert.Equal(t, consumerID, summaries[0].Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].Counterpart.Role)
	assert.Equal(t, "Ana", summaries[0].Counterpart.Name)
	assert.Equal(t, "Pérez", summaries[0].Counterpart.Surname)
	assert.Empty(t, summaries[0].Counterpart.CategoryName)
	require.NotNil(t, summaries[0].LastMessage)
	assert.Equal(t, conversation.SenderConsumer, summaries[0].LastMessage.SenderRole)
	assert.Equal(t, messageToSave.Content, summaries[0].LastMessage.Content)
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
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	detail, err := reader.FindDetailByIDForConsumer(context.Background(), savedConversation.ID)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, savedConversation.ID, detail.ID)
	assert.Equal(t, conversation.StatusPending, detail.Status)
	assert.Equal(t, providerID, detail.Counterpart.ID)
	assert.Equal(t, conversation.SenderProvider, detail.Counterpart.Role)
	assert.Equal(t, "Juan", detail.Counterpart.Name)
	assert.Equal(t, "Gómez", detail.Counterpart.Surname)
	assert.Equal(t, "Plomería", detail.Counterpart.CategoryName)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, detail.Messages[0].SenderRole)
	assert.Equal(t, messageToSave.Content, detail.Messages[0].Content)
	assert.NotZero(t, detail.Messages[0].CreatedOn)
	assert.NotZero(t, detail.UpdatedOn)
}

func TestConversationReaderFindsDetailForProvider(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database)
	consumerID, providerID := savedConversationParticipants(t, testContext)
	conversationToSave, messageToSave := pendingConversationWithMessage(t, consumerID, providerID)
	savedConversation, err := testContext.conversationRepository.SaveWithMessage(conversationToSave, messageToSave)
	require.NoError(t, err)

	detail, err := reader.FindDetailByIDForProvider(context.Background(), savedConversation.ID)

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, savedConversation.ID, detail.ID)
	assert.Equal(t, conversation.StatusPending, detail.Status)
	assert.Equal(t, consumerID, detail.Counterpart.ID)
	assert.Equal(t, conversation.SenderConsumer, detail.Counterpart.Role)
	assert.Equal(t, "Ana", detail.Counterpart.Name)
	assert.Equal(t, "Pérez", detail.Counterpart.Surname)
	assert.Empty(t, detail.Counterpart.CategoryName)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, conversation.SenderConsumer, detail.Messages[0].SenderRole)
	assert.Equal(t, messageToSave.Content, detail.Messages[0].Content)
	assert.NotZero(t, detail.Messages[0].CreatedOn)
	assert.NotZero(t, detail.UpdatedOn)
}

func TestConversationReaderReturnsNotFoundForMissingConversationDetail(t *testing.T) {
	testContext := newConversationRepositoryTest(t)
	reader := repositories.NewConversationReader(testContext.database)

	detail, err := reader.FindDetailByIDForConsumer(context.Background(), 999)

	assert.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
	assert.Nil(t, detail)
}
