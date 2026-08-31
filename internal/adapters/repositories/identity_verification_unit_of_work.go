package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
)

type IdentityVerificationUnitOfWork struct {
	db         *sql.DB
	repository *IdentityVerificationRepository
}

func NewIdentityVerificationUnitOfWork(db *sql.DB, repository *IdentityVerificationRepository) *IdentityVerificationUnitOfWork {
	return &IdentityVerificationUnitOfWork{db: db, repository: repository}
}

func (unit *IdentityVerificationUnitOfWork) Execute(ctx context.Context, operation func(identityverification.TransactionalStore) error) error {
	if operation == nil {
		return fmt.Errorf("executing identity verification unit of work: operation is required")
	}
	tx, err := unit.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning identity verification unit of work: %w", err)
	}
	store := &identityVerificationTransactionalStore{repository: unit.repository, tx: tx}
	if err := operation(store); err != nil {
		return rollbackIdentityVerificationUnitOfWork(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing identity verification unit of work: %w", err)
	}
	return nil
}

type identityVerificationTransactionalStore struct {
	repository *IdentityVerificationRepository
	tx         *sql.Tx
}

func (store *identityVerificationTransactionalStore) SaveVerification(ctx context.Context, verification *identityverification.IdentityVerification) error {
	return store.repository.saveWithExecutor(ctx, store.tx, verification)
}

func (store *identityVerificationTransactionalStore) FindVerificationBySessionID(ctx context.Context, sessionID uuid.UUID) (*identityverification.IdentityVerification, error) {
	return store.repository.findBySessionIDWithExecutor(ctx, store.tx, sessionID)
}

func (store *identityVerificationTransactionalStore) SaveEvent(ctx context.Context, event *identityverification.VerificationEvent) (bool, error) {
	return store.repository.saveEventWithExecutor(ctx, store.tx, event)
}

func rollbackIdentityVerificationUnitOfWork(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back identity verification unit of work: %v", cause, rollbackErr)
	}
	return cause
}
