package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (repository *MessageRepository) ExistsInConversation(conversationID int, content string) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM messages WHERE conversation_id = $1 AND content = $2
		)`,
		conversationID,
		content,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking message existence in conversation: %w", err)
	}

	return exists, nil
}

func (repository *MessageRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM messages`)
	if err != nil {
		return fmt.Errorf("deleting all messages: %w", err)
	}

	return nil
}

func (repository *MessageRepository) saveWithTx(ctx context.Context, tx *sql.Tx, conversationID int, message conversation.Message) (*conversation.Message, error) {
	var savedMessage conversation.Message
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO messages (conversation_id, sender_role, content, created_on)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, conversation_id, sender_role, content, created_on`,
		conversationID,
		message.SenderRole,
		message.Content,
	).Scan(
		&savedMessage.ID,
		&savedMessage.ConversationID,
		&savedMessage.SenderRole,
		&savedMessage.Content,
		&savedMessage.CreatedOn,
	)
	if err != nil {
		return nil, fmt.Errorf("saving message: %w", err)
	}

	return &savedMessage, nil
}
