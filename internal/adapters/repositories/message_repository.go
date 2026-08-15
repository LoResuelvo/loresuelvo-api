package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

type MessageRepository struct {
	db              *sql.DB
	imageRepository *MessageImageRepository
	audioRepository *MessageAudioRepository
}

func NewMessageRepository(db *sql.DB, imageRepository *MessageImageRepository, audioRepository *MessageAudioRepository) *MessageRepository {
	return &MessageRepository{db: db, imageRepository: imageRepository, audioRepository: audioRepository}
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

func (repository *MessageRepository) CountByConversationIDAndSenderRole(conversationID int, senderRole string) (int, error) {
	var count int
	err := repository.db.QueryRow(
		`SELECT COUNT(*)
		FROM messages
		WHERE conversation_id = $1 AND sender_role = $2`,
		conversationID,
		senderRole,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting messages in conversation by sender role: %w", err)
	}

	return count, nil
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

	if message.Audio != nil && len(message.Images) > 0 {
		return nil, conversation.ErrMessageAudioMustBeExclusive
	}
	savedMessage.Images = append(savedMessage.Images, message.Images...)
	if err := repository.imageRepository.saveWithTx(ctx, tx, savedMessage.ID, savedMessage.Images); err != nil {
		return nil, err
	}
	savedMessage.Audio = message.Audio
	if err := repository.audioRepository.saveWithTx(ctx, tx, savedMessage.ID, savedMessage.Audio); err != nil {
		return nil, err
	}

	return &savedMessage, nil
}

func (repository *MessageRepository) findImagesByConversationID(ctx context.Context, conversationID int) (map[int][]persistedMessageImage, error) {
	return repository.imageRepository.findByConversationID(ctx, conversationID)
}

func (repository *MessageRepository) findAudiosByConversationID(ctx context.Context, conversationID int) (map[int]persistedMessageAudio, error) {
	return repository.audioRepository.findByConversationID(ctx, conversationID)
}
