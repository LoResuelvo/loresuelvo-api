package conversation_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPendingConversationCreatesPendingConversation(t *testing.T) {
	pendingConversation, err := conversation.NewPendingConversation(10, 20)

	require.NoError(t, err)
	require.NotNil(t, pendingConversation)
	workConversation := pendingConversation.(*conversation.WorkConversation)
	assert.Equal(t, conversation.TypeWork, workConversation.ConversationType())
	assert.Equal(t, 10, workConversation.ConsumerID)
	assert.Equal(t, 20, workConversation.ProviderID)
	assert.Equal(t, conversation.StatusPending, workConversation.Status())
}

func TestNewPendingConversationRejectsMissingConsumerID(t *testing.T) {
	pendingConversation, err := conversation.NewPendingConversation(0, 20)

	assert.ErrorIs(t, err, conversation.ErrConsumerRequired)
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
	assert.Equal(t, conversation.StatusActive, typedConversation.Status())
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
	assert.Equal(t, conversation.StatusActive, pendingConversation.Status())
}

func TestCannotActivateNonPendingConversation(t *testing.T) {
	pendingConversation, _ := conversation.NewPendingConversation(10, 20)
	_ = pendingConversation.Activate()

	err := pendingConversation.Activate()

	assert.ErrorIs(t, err, conversation.ErrOnlyPendingConversationCanBeActivated)
	assert.Equal(t, conversation.StatusActive, pendingConversation.Status())
}

func TestChatbotConversationRejectsInvalidContext(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)

	err = chatbotConversation.(*conversation.ChatBotConversation).UpdateContext(conversation.ChatbotConversationContext{
		Summary:                 "Resumen",
		LastSummarizedMessageID: -1,
	})

	assert.ErrorIs(t, err, conversation.ErrChatbotContextInvalid)
}

func TestChatbotConversationUpdatesContext(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)

	err = typedConversation.UpdateContext(conversation.ChatbotConversationContext{
		Summary:                 "  Resumen de la conversación  ",
		LastSummarizedMessageID: 5,
	})

	require.NoError(t, err)
	assert.Equal(t, "Resumen de la conversación", typedConversation.Context.Summary)
	assert.Equal(t, 5, typedConversation.Context.LastSummarizedMessageID)
}

func TestChatbotConversationManagesProcessingState(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)
	now := time.Now().UTC()

	require.NoError(t, typedConversation.StartProcessing(now))
	startedAt := typedConversation.ProcessingStartedAt()
	require.NotNil(t, startedAt)
	assert.Equal(t, now, *startedAt)

	err = typedConversation.StartProcessing(now.Add(conversation.ChatbotProcessingStaleAfter / 2))
	assert.ErrorIs(t, err, conversation.ErrChatbotConversationAlreadyProcessing)

	require.NoError(t, typedConversation.StartProcessing(now.Add(conversation.ChatbotProcessingStaleAfter)))
	typedConversation.FinishProcessing()
	assert.Nil(t, typedConversation.ProcessingStartedAt())
}

func TestChatbotConversationAddsValidTurn(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)
	consumerMessage, err := conversation.NewConsumerMessage("Sale agua del sifón")
	require.NoError(t, err)
	chatbotMessage, err := conversation.NewChatbotMessage("Revisá la rosca del sifón")
	require.NoError(t, err)

	err = typedConversation.AddTurn(*consumerMessage, *chatbotMessage)

	require.NoError(t, err)
	require.Len(t, typedConversation.Messages(), 2)
	assert.Equal(t, conversation.SenderConsumer, typedConversation.Messages()[0].SenderRole)
	assert.Equal(t, conversation.SenderChatbot, typedConversation.Messages()[1].SenderRole)
}

func TestChatbotConversationRejectsInvalidTurn(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)
	chatbotMessage, err := conversation.NewChatbotMessage("Respuesta")
	require.NoError(t, err)

	err = typedConversation.AddTurn(*chatbotMessage, *chatbotMessage)

	assert.ErrorIs(t, err, conversation.ErrInvalidChatbotTurn)
	assert.Empty(t, typedConversation.Messages())
}

func TestChatbotConversationReturnsRecentMessagesCopy(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)
	for _, content := range []string{"uno", "dos", "tres"} {
		message, err := conversation.NewConsumerMessage(content)
		require.NoError(t, err)
		typedConversation.AddMessage(*message)
	}

	recentMessages := typedConversation.RecentMessages(2)

	require.Len(t, recentMessages, 2)
	assert.Equal(t, "dos", recentMessages[0].Content)
	assert.Equal(t, "tres", recentMessages[1].Content)
	recentMessages[0].Content = "mutado"
	assert.Equal(t, "dos", typedConversation.Messages()[1].Content)
}

func TestChatbotConversationReturnsMessagesPendingSummary(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	typedConversation := chatbotConversation.(*conversation.ChatBotConversation)
	require.NoError(t, typedConversation.UpdateContext(conversation.ChatbotConversationContext{LastSummarizedMessageID: 2}))
	for messageID := 1; messageID <= 4; messageID++ {
		typedConversation.AddMessage(conversation.Message{ID: messageID, SenderRole: conversation.SenderConsumer, Content: "mensaje"})
	}

	pendingMessages := typedConversation.MessagesPendingSummary()

	require.Len(t, pendingMessages, 2)
	assert.Equal(t, 3, pendingMessages[0].ID)
	assert.Equal(t, 4, pendingMessages[1].ID)
	assert.True(t, typedConversation.ShouldSummarizeContext(2))
	assert.False(t, typedConversation.ShouldSummarizeContext(3))
}

func TestConversationReturnsLastMessage(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)
	firstMessage, err := conversation.NewConsumerMessage("Primer mensaje")
	require.NoError(t, err)
	lastMessage, err := conversation.NewChatbotMessage("Último mensaje")
	require.NoError(t, err)
	chatbotConversation.AddMessage(*firstMessage)
	chatbotConversation.AddMessage(*lastMessage)

	foundMessage, ok := chatbotConversation.LastMessage()

	require.True(t, ok)
	assert.Equal(t, conversation.SenderChatbot, foundMessage.SenderRole)
	assert.Equal(t, "Último mensaje", foundMessage.Content)
}

func TestConversationLastMessageReturnsFalseWhenEmpty(t *testing.T) {
	chatbotConversation, err := conversation.NewChatbotConversation(10, "Pérdida de agua")
	require.NoError(t, err)

	foundMessage, ok := chatbotConversation.LastMessage()

	assert.False(t, ok)
	assert.Empty(t, foundMessage)
}

func TestBaseConversationIsActiveReturnsTrueForActiveConversation(t *testing.T) {
	conv := conversation.NewBaseConversation(conversation.TypeWork, conversation.StatusActive)
	assert.True(t, conv.IsActive())
}

func TestBaseConversationIsActiveReturnsFalseForInactiveConversation(t *testing.T) {
	conv := conversation.NewBaseConversation(conversation.TypeWork, conversation.StatusPending)
	assert.False(t, conv.IsActive())
}
