package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	"github.com/stretchr/testify/require"
)

func TestConversationReaderFindsWorkAccess(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	access, err := fixture.reader.FindAccessByID(context.Background(), fixture.savedConversation.ID())
	require.NoError(t, err)
	require.Equal(t, &readmodel.ConversationAccess{ID: fixture.savedConversation.ID(), Type: conversation.TypeWork, ConsumerID: fixture.consumerID, ProviderID: fixture.providerID}, access)
}

func TestConversationReaderFindsChatbotAccess(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	chatbot, err := conversation.NewChatbotConversation(fixture.consumerID, "Water leak")
	require.NoError(t, err)
	saved, err := fixture.conversationRepository.SaveConversation(context.Background(), chatbot)
	require.NoError(t, err)
	access, err := fixture.reader.FindAccessByID(context.Background(), saved.ID())
	require.NoError(t, err)
	require.Equal(t, &readmodel.ConversationAccess{ID: saved.ID(), Type: conversation.TypeChatbot, ConsumerID: fixture.consumerID}, access)
}

func TestConversationReaderAccessPreservesFailures(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	t.Run("missing", func(t *testing.T) {
		access, err := fixture.reader.FindAccessByID(context.Background(), -1)
		require.ErrorIs(t, err, conversation.ErrConversationDoesNotExist)
		require.Nil(t, access)
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		access, err := fixture.reader.FindAccessByID(ctx, fixture.savedConversation.ID())
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, access)
	})
	t.Run("scan", func(t *testing.T) {
		// A header without either subtype exposes a NULL consumer, matching the aggregate reader's scan failure.
		var id int
		err := fixture.database.QueryRow("INSERT INTO conversations (type, status) VALUES ($1, $2) RETURNING id", conversation.TypeWork, conversation.StatusPending).Scan(&id)
		require.NoError(t, err)
		access, err := fixture.reader.FindAccessByID(context.Background(), id)
		require.ErrorContains(t, err, "Scan error")
		require.False(t, errors.Is(err, conversation.ErrConversationDoesNotExist))
		require.Nil(t, access)
	})
	t.Run("query", func(t *testing.T) {
		database, err := sql.Open("pgx", "postgres://unused")
		require.NoError(t, err)
		require.NoError(t, database.Close())
		reader := repositories.NewConversationReader(database, nil, nil)
		access, err := reader.FindAccessByID(context.Background(), 1)
		require.ErrorContains(t, err, "database is closed")
		require.Nil(t, access)
	})
}
