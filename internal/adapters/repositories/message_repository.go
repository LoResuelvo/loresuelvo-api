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

func (repository *MessageRepository) FindLastMessagesByConversationIDs(ctx context.Context, conversationIDs []int) (map[int]conversation.Message, error) {
	if len(conversationIDs) == 0 {
		return nil, nil
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT m.id, m.conversation_id, m.sender_role, m.content, m.created_on
		FROM messages m
		INNER JOIN (
			SELECT id, conversation_id, MAX(created_on) as max_created
			FROM messages
			WHERE conversation_id = ANY($1)
			GROUP BY id, conversation_id
		) latest ON m.id = latest.id AND m.conversation_id = latest.conversation_id
		WHERE m.conversation_id = ANY($1)`,
		conversationIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("finding last messages by conversation ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int]conversation.Message)
	for rows.Next() {
		var msg conversation.Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderRole, &msg.Content, &msg.CreatedOn); err != nil {
			return nil, fmt.Errorf("scanning last message: %w", err)
		}
		result[msg.ConversationID] = msg
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating last messages: %w", err)
	}

	return result, nil
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
