package operation_test

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChatQueryValidatesContinuationAndLimits(t *testing.T) {
	require.Equal(t, 20, (operation.ChatQuery{}).EffectiveLimit())
	for _, query := range []operation.ChatQuery{{Limit: -1}, {Limit: 101}, {ConversationID: 9}, {After: &conversation.MessagePosition{}}, {ConversationID: 9, After: &conversation.MessagePosition{ID: 1}}} {
		require.ErrorIs(t, query.Validate(), operation.ErrInvalidChatQuery)
	}
	require.NoError(t, (operation.ChatQuery{Limit: 100, ConversationID: 9, After: &conversation.MessagePosition{ID: 1, CreatedOn: time.Now()}}).Validate())
}
