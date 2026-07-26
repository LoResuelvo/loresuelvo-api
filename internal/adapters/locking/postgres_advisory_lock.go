package locking

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
)

type PostgresAdvisoryLock struct {
	db *sql.DB
}

func NewPostgresAdvisoryLock(db *sql.DB) *PostgresAdvisoryLock {
	return &PostgresAdvisoryLock{db: db}
}

func (lock *PostgresAdvisoryLock) WithinLock(
	ctx context.Context,
	key payment.LockKey,
	operation func() error,
) error {
	if key.Namespace <= 0 || key.Resource == "" || operation == nil {
		return fmt.Errorf("acquiring advisory lock: key and operation are required")
	}
	tx, err := lock.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning advisory lock transaction: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock($1, hashtext($2))`,
		key.Namespace,
		key.Resource,
	); err != nil {
		return rollbackAdvisoryLock(tx, fmt.Errorf("acquiring advisory lock: %w", err))
	}
	if err := operation(); err != nil {
		return rollbackAdvisoryLock(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing advisory lock transaction: %w", err)
	}
	return nil
}

func rollbackAdvisoryLock(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back advisory lock: %v", cause, rollbackErr)
	}
	return cause
}
