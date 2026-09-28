package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"time"
)

// OperationChatFixture persists exact historical instants for administrative read tests.
type OperationChatFixture struct{ DB *sql.DB }

func (fixture OperationChatFixture) AddMessage(ctx context.Context, conversationID int, role, content string, created time.Time) (conversation.Message, error) {
	message := conversation.Message{ConversationID: conversationID, SenderRole: role, Content: content, CreatedOn: created}
	if role != conversation.SenderConsumer && role != conversation.SenderProvider {
		return message, fmt.Errorf("unsupported participant role %q", role)
	}
	err := fixture.DB.QueryRowContext(ctx, `INSERT INTO messages (conversation_id,sender_role,content,created_on) VALUES ($1,$2,$3,$4) RETURNING id`, conversationID, role, content, created).Scan(&message.ID)
	if err != nil {
		return message, fmt.Errorf("persisting historical chat message: %w", err)
	}
	return message, nil
}
