package conversation_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/assert"
)

func TestConversationIsActiveReturnsTrueForActiveConversation(t *testing.T) {
	conv, _ := conversation.NewPendingConversation(1, 2)
	_ = conv.Activate()

	assert.True(t, conv.IsActive())
}

func TestConversationIsActiveReturnsFalseForInactiveConversation(t *testing.T) {
	conv, err := conversation.NewPendingConversation(1, 2)

	assert.NoError(t, err)
	assert.False(t, conv.IsActive())
}
