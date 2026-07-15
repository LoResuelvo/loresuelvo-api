package conversation_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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

func TestNewChatbotMessageCreatesTrimmedChatbotMessage(t *testing.T) {
	message, err := conversation.NewChatbotMessage("  Revisá el sifón y cerrá la llave de paso.  ")

	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, conversation.SenderChatbot, message.SenderRole)
	assert.Equal(t, "Revisá el sifón y cerrá la llave de paso.", message.Content)
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

func TestNewConsumerMessageWithImagesAllowsEmptyText(t *testing.T) {
	message, err := conversation.NewConsumerMessage("   ", filedomain.MessageImage{Image: filedomain.Image{FileID: "file-id", OriginalName: "problem.jpg"}})

	require.NoError(t, err)
	assert.Empty(t, message.Content)
	require.Len(t, message.Images, 1)
	assert.Equal(t, "file-id", message.Images[0].FileID)
}

func TestNewConsumerMessageWithImagesRejectsDuplicateFiles(t *testing.T) {
	message, err := conversation.NewConsumerMessage("Problem", filedomain.MessageImage{Image: filedomain.Image{FileID: "file-id"}}, filedomain.MessageImage{Image: filedomain.Image{FileID: "file-id"}})

	assert.Nil(t, message)
	assert.ErrorIs(t, err, conversation.ErrMessageImageNotAvailable)
}
