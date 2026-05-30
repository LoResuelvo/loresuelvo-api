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

func (repository *MessageRepository) FindByConversationID(ctx context.Context, conversationID int) ([]conversation.Message, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, conversation_id, sender_role, content
		FROM messages
		WHERE conversation_id = $1
		ORDER BY id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding messages by conversation id: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	messages := []conversation.Message{}
	for rows.Next() {
		var message conversation.Message
		if err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.SenderRole,
			&message.Content,
		); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}

	return messages, nil
}

func (repository *MessageRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM messages`)
	if err != nil {
		return fmt.Errorf("deleting all messages: %w", err)
	}

	return nil
}

func (repository *MessageRepository) saveWithTx(tx *sql.Tx, conversationID int, message conversation.Message) (*conversation.Message, error) {
	var savedMessage conversation.Message
	err := tx.QueryRow(
		`INSERT INTO messages (conversation_id, sender_role, content, created_on)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, conversation_id, sender_role, content`,
		conversationID,
		message.SenderRole,
		message.Content,
	).Scan(
		&savedMessage.ID,
		&savedMessage.ConversationID,
		&savedMessage.SenderRole,
		&savedMessage.Content,
	)
	if err != nil {
		return nil, fmt.Errorf("saving message: %w", err)
	}

	return &savedMessage, nil
}
