package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/jackc/pgx/v5/pgconn"
)

type JobRequestRepository struct {
	db *sql.DB
}

func NewJobRequestRepository(db *sql.DB) *JobRequestRepository {
	return &JobRequestRepository{db: db}
}

func (repository *JobRequestRepository) SaveWithConversation(jobRequest jobrequest.JobRequest, pendingConversation conversation.Conversation) (*jobrequest.JobRequest, error) {
	ctx := context.Background()
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning job request transaction: %w", err)
	}

	var conversationID int
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO conversations (consumer_id, provider_id, status, created_on, updated_on)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id`,
		pendingConversation.ConsumerID,
		pendingConversation.ProviderID,
		pendingConversation.Status,
	).Scan(&conversationID)
	if err != nil {
		return nil, rollbackJobRequestTx(tx, mapJobRequestInsertError(err))
	}

	var savedJobRequest jobrequest.JobRequest
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO job_requests (consumer_id, provider_id, conversation_id, title, description, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, consumer_id, provider_id, conversation_id, title, description`,
		jobRequest.ConsumerID,
		jobRequest.ProviderID,
		conversationID,
		jobRequest.Title,
		jobRequest.Description,
	).Scan(
		&savedJobRequest.ID,
		&savedJobRequest.ConsumerID,
		&savedJobRequest.ProviderID,
		&savedJobRequest.ConversationID,
		&savedJobRequest.Title,
		&savedJobRequest.Description,
	)
	if err != nil {
		return nil, rollbackJobRequestTx(tx, mapJobRequestInsertError(err))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing job request transaction: %w", err)
	}

	return &savedJobRequest, nil
}

func (repository *JobRequestRepository) FindByConversationID(conversationID int) (*jobrequest.JobRequest, error) {
	var foundJobRequest jobrequest.JobRequest
	err := repository.db.QueryRow(
		`SELECT id, consumer_id, provider_id, conversation_id, title, description
		FROM job_requests
		WHERE conversation_id = $1`,
		conversationID,
	).Scan(
		&foundJobRequest.ID,
		&foundJobRequest.ConsumerID,
		&foundJobRequest.ProviderID,
		&foundJobRequest.ConversationID,
		&foundJobRequest.Title,
		&foundJobRequest.Description,
	)
	if err != nil {
		return nil, fmt.Errorf("finding job request by conversation id: %w", err)
	}

	return &foundJobRequest, nil
}

func (repository *JobRequestRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM job_requests`)
	if err != nil {
		return fmt.Errorf("deleting all job requests: %w", err)
	}

	return nil
}

func rollbackJobRequestTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback job request transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}

func mapJobRequestInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return jobrequest.ErrAlreadyExists
	}

	return fmt.Errorf("saving job request: %w", err)
}
