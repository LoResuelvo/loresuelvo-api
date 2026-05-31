package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolationCode = "23505"

type ConversationRepository struct {
	db                *sql.DB
	messageRepository *MessageRepository
}

func NewConversationRepository(db *sql.DB, messageRepository *MessageRepository) *ConversationRepository {
	return &ConversationRepository{db: db, messageRepository: messageRepository}
}

func (repository *ConversationRepository) ExistsBetween(consumerID, providerID int) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM conversations WHERE consumer_id = $1 AND provider_id = $2
		)`,
		consumerID,
		providerID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking conversation existence: %w", err)
	}

	return exists, nil
}

func (repository *ConversationRepository) SaveWithMessage(conversationToSave conversation.Conversation, message conversation.Message) (*conversation.Conversation, error) {
	tx, err := repository.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning conversation transaction: %w", err)
	}

	var savedConversation conversation.Conversation
	err = tx.QueryRow(
		`INSERT INTO conversations (consumer_id, provider_id, status, created_on, updated_on)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, consumer_id, provider_id, status`,
		conversationToSave.ConsumerID,
		conversationToSave.ProviderID,
		conversationToSave.Status,
	).Scan(
		&savedConversation.ID,
		&savedConversation.ConsumerID,
		&savedConversation.ProviderID,
		&savedConversation.Status,
	)
	if err != nil {
		return nil, rollbackConversationTx(tx, mapConversationInsertError(err))
	}

	savedMessage, err := repository.messageRepository.saveWithTx(tx, savedConversation.ID, message)
	if err != nil {
		return nil, rollbackConversationTx(tx, err)
	}
	savedConversation.Messages = []conversation.Message{*savedMessage}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing conversation transaction: %w", err)
	}

	return &savedConversation, nil
}

func (repository *ConversationRepository) DeleteBetween(consumerID, providerID int) error {
	_, err := repository.db.Exec(
		`DELETE FROM conversations WHERE consumer_id = $1 AND provider_id = $2`,
		consumerID,
		providerID,
	)
	if err != nil {
		return fmt.Errorf("deleting conversation between consumer and provider: %w", err)
	}

	return nil
}

func (repository *ConversationRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM conversations`)
	if err != nil {
		return fmt.Errorf("deleting all conversations: %w", err)
	}

	return nil
}

func (repository *ConversationRepository) FindBetween(consumerID, providerID int) (*conversation.Conversation, error) {
	var foundConversation conversation.Conversation
	err := repository.db.QueryRow(
		`SELECT id, consumer_id, provider_id, status
		FROM conversations
		WHERE consumer_id = $1 AND provider_id = $2`,
		consumerID,
		providerID,
	).Scan(
		&foundConversation.ID,
		&foundConversation.ConsumerID,
		&foundConversation.ProviderID,
		&foundConversation.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("finding conversation between consumer and provider: %w", err)
	}

	return &foundConversation, nil
}

func (repository *ConversationRepository) FindByID(ctx context.Context, conversationID int) (*conversation.Conversation, error) {
	var foundConversation conversation.Conversation
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, consumer_id, provider_id, status
		FROM conversations
		WHERE id = $1`,
		conversationID,
	).Scan(
		&foundConversation.ID,
		&foundConversation.ConsumerID,
		&foundConversation.ProviderID,
		&foundConversation.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, conversation.ErrConversationDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding conversation by id: %w", err)
	}

	return &foundConversation, nil
}

func rollbackConversationTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback conversation transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}

func mapConversationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return conversation.ErrAlreadyExists
	}

	return fmt.Errorf("saving conversation: %w", err)
}
