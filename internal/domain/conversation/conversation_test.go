package conversation_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPendingConversationCreatesPendingConversation(t *testing.T) {
	pendingConversation, err := conversation.NewPendingConversation(10, 20)

	require.NoError(t, err)
	require.NotNil(t, pendingConversation)
	assert.Equal(t, 10, pendingConversation.ConsumerID)
	assert.Equal(t, 20, pendingConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, pendingConversation.Status)
}

func TestNewPendingConversationRejectsMissingConsumerID(t *testing.T) {
	pendingConversation, err := conversation.NewPendingConversation(0, 20)

	assert.ErrorIs(t, err, conversation.ErrOnlyConsumerCanStartWorkRequest)
	assert.Nil(t, pendingConversation)
}

func TestNewPendingConversationRejectsMissingProviderID(t *testing.T) {
	pendingConversation, err := conversation.NewPendingConversation(10, 0)

	assert.ErrorIs(t, err, conversation.ErrProviderRequired)
	assert.Nil(t, pendingConversation)
}

func TestCanActivateAConversation(t *testing.T) {
	pendingConversation, _ := conversation.NewPendingConversation(10, 20)
	err := pendingConversation.Activate()
	require.NoError(t, err)
	assert.Equal(t, conversation.StatusActive, pendingConversation.Status)
}

func TestCannotActivateNonPendingConversation(t *testing.T) {
	pendingConversation, _ := conversation.NewPendingConversation(10, 20)
	_ = pendingConversation.Activate()

	err := pendingConversation.Activate()

	assert.ErrorIs(t, err, conversation.ErrOnlyPendingConversationCanBeActivated)
	assert.Equal(t, conversation.StatusActive, pendingConversation.Status)
}
