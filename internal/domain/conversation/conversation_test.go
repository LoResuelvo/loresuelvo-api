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
	workConversation := pendingConversation.(*conversation.WorkConversation)
	assert.Equal(t, 10, workConversation.ConsumerID)
	assert.Equal(t, 20, workConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, workConversation.Base().Status)
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

func TestNewChatbotConversationCreatesActiveConversation(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua en la cocina")

	require.NoError(t, err)
	require.NotNil(t, chatbotConversation)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)
	assert.Equal(t, conversation.TypeChatbot, typedConversation.ConversationType())
	assert.Equal(t, 10, typedConversation.ConsumerID)
	assert.Equal(t, "Pérdida de agua en la cocina", typedConversation.Title)
	assert.Equal(t, conversation.StatusActive, typedConversation.Base().Status)
}

func TestConversationCanAddMessages(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua en la cocina")
	require.NoError(t, err)
	message, err := conversation.NewConsumerMessage("Tengo una pérdida de agua")
	require.NoError(t, err)

	chatbotConversation.AddMessage(*message)

	require.Len(t, chatbotConversation.Messages(), 1)
	assert.Equal(t, conversation.SenderConsumer, chatbotConversation.Messages()[0].SenderRole)
	assert.Equal(t, "Tengo una pérdida de agua", chatbotConversation.Messages()[0].Content)
}

func TestCanActivateAConversation(t *testing.T) {
	pendingConversation, _ := conversation.NewPendingConversation(10, 20)
	err := pendingConversation.Activate()
	require.NoError(t, err)
	assert.Equal(t, conversation.StatusActive, pendingConversation.Base().Status)
}

func TestCannotActivateNonPendingConversation(t *testing.T) {
	pendingConversation, _ := conversation.NewPendingConversation(10, 20)
	_ = pendingConversation.Activate()

	err := pendingConversation.Activate()

	assert.ErrorIs(t, err, conversation.ErrOnlyPendingConversationCanBeActivated)
	assert.Equal(t, conversation.StatusActive, pendingConversation.Base().Status)
}
