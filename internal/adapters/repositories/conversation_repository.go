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
	ctx := context.Background()
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning conversation transaction: %w", err)
	}

	var savedConversation conversation.Conversation
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO conversations (consumer_id, provider_id, status, created_on, updated_on)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, consumer_id, provider_id, status, updated_on`,
		conversationToSave.ConsumerID,
		conversationToSave.ProviderID,
		conversationToSave.Status,
	).Scan(
		&savedConversation.ID,
		&savedConversation.ConsumerID,
		&savedConversation.ProviderID,
		&savedConversation.Status,
		&savedConversation.UpdatedOn,
	)
	if err != nil {
		return nil, rollbackConversationTx(tx, mapConversationInsertError(err))
	}

	savedMessage, err := repository.messageRepository.saveWithTx(ctx, tx, savedConversation.ID, message)
	if err != nil {
		return nil, rollbackConversationTx(tx, err)
	}
	savedConversation.Messages = []conversation.Message{*savedMessage}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing conversation transaction: %w", err)
	}

	return &savedConversation, nil
}

func (repository *ConversationRepository) AddMessage(ctx context.Context, conversationID int, message conversation.Message) (*conversation.Message, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning add message transaction: %w", err)
	}

	if err := lockConversationByID(ctx, tx, conversationID); err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	savedMessage, err := repository.messageRepository.saveWithTx(ctx, tx, conversationID, message)
	if err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE conversations
		SET updated_on = $2
		WHERE id = $1`,
		conversationID,
		savedMessage.CreatedOn,
	)
	if err != nil {
		return nil, rollbackConversationTx(tx, fmt.Errorf("updating conversation timestamp after message: %w", err))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, rollbackConversationTx(tx, fmt.Errorf("checking updated conversation rows: %w", err))
	}
	if rowsAffected == 0 {
		return nil, rollbackConversationTx(tx, conversation.ErrConversationDoesNotExist)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing add message transaction: %w", err)
	}

	return savedMessage, nil
}

func lockConversationByID(ctx context.Context, tx *sql.Tx, conversationID int) error {
	var id int
	err := tx.QueryRowContext(
		ctx,
		`SELECT id
		FROM conversations
		WHERE id = $1
		FOR UPDATE`,
		conversationID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrConversationDoesNotExist
	}
	if err != nil {
		return fmt.Errorf("locking conversation by id: %w", err)
	}

	return nil
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
		`SELECT id, consumer_id, provider_id, status, updated_on
		FROM conversations
		WHERE consumer_id = $1 AND provider_id = $2`,
		consumerID,
		providerID,
	).Scan(
		&foundConversation.ID,
		&foundConversation.ConsumerID,
		&foundConversation.ProviderID,
		&foundConversation.Status,
		&foundConversation.UpdatedOn,
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
		`SELECT id, consumer_id, provider_id, status, updated_on
		FROM conversations
		WHERE id = $1`,
		conversationID,
	).Scan(
		&foundConversation.ID,
		&foundConversation.ConsumerID,
		&foundConversation.ProviderID,
		&foundConversation.Status,
		&foundConversation.UpdatedOn,
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
