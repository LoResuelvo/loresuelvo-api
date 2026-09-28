package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

const MaxConversationMessagePageSize = 100

type ConversationMessagePageReader struct{ db *sql.DB }

func NewConversationMessagePageReader(db *sql.DB) *ConversationMessagePageReader {
	return &ConversationMessagePageReader{db: db}
}

// Only message text is projected: legacy attachment URLs and storage locations are never read.
const conversationMessagePageSQL = `SELECT id, conversation_id, sender_role, content, created_on FROM messages
 WHERE conversation_id = $1 ORDER BY created_on ASC, id ASC LIMIT $2`

func (reader *ConversationMessagePageReader) FindPage(ctx context.Context, conversationID, limit int) ([]conversation.Message, error) {
	if conversationID <= 0 || limit < 1 || limit > MaxConversationMessagePageSize {
		return nil, fmt.Errorf("reading conversation message page: %w", conversation.ErrInvalidMessagePage)
	}
	rows, err := reader.db.QueryContext(ctx, conversationMessagePageSQL, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying conversation message page: %w", err)
	}
	defer rows.Close()
	messages := make([]conversation.Message, 0, limit)
	for rows.Next() {
		var message conversation.Message
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.SenderRole, &message.Content, &message.CreatedOn); err != nil {
			return nil, fmt.Errorf("scanning conversation message page: %w", err)
		}
		message.CreatedOn = message.CreatedOn.UTC()
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating conversation message page: %w", err)
	}
	return messages, nil
}
