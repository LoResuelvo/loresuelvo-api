package conversation_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumerMessageCreatesTrimmedConsumerMessage(t *testing.T) {
	message, err := conversation.NewConsumerMessage("  Hola, necesito un presupuesto  ")

	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, conversation.SenderConsumer, message.SenderRole)
	assert.Equal(t, "Hola, necesito un presupuesto", message.Content)
	assert.Zero(t, message.ConversationID)
}

func TestNewProviderMessageCreatesTrimmedProviderMessage(t *testing.T) {
	message, err := conversation.NewProviderMessage("  Sí, puedo pasar el jueves  ")

	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, conversation.SenderProvider, message.SenderRole)
	assert.Equal(t, "Sí, puedo pasar el jueves", message.Content)
	assert.Zero(t, message.ConversationID)
}

func TestNewConsumerMessageRejectsEmptyContent(t *testing.T) {
	message, err := conversation.NewConsumerMessage("   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, message)
}

func TestNewProviderMessageRejectsEmptyContent(t *testing.T) {
	message, err := conversation.NewProviderMessage("   ")

	assert.ErrorIs(t, err, conversation.ErrMessageRequired)
	assert.Nil(t, message)
}
