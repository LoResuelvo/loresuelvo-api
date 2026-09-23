package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
)

// CategoryUnitOfWork commits a category and its audit event together.
type CategoryUnitOfWork struct {
	db          *sql.DB
	categories  *CategoryRepository
	auditEvents *AuditEventRepository
}

func NewCategoryUnitOfWork(db *sql.DB, categories *CategoryRepository, auditEvents *AuditEventRepository) *CategoryUnitOfWork {
	return &CategoryUnitOfWork{db: db, categories: categories, auditEvents: auditEvents}
}

func (unit *CategoryUnitOfWork) Execute(ctx context.Context, operation func(category.TransactionalStore) error) error {
	if operation == nil {
		return fmt.Errorf("executing category unit of work: operation is required")
	}
	tx, err := unit.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning category unit of work: %w", err)
	}
	// A panic in the caller's operation must release the transaction as well.
	// Explicit rollback below still preserves rollback errors on ordinary failures.
	defer func() { _ = tx.Rollback() }()
	store := &categoryTransactionalStore{tx: tx, categories: unit.categories, auditEvents: unit.auditEvents}
	if err := operation(store); err != nil {
		return rollbackCategoryUnitOfWork(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing category unit of work: %w", err)
	}
	return nil
}

type categoryTransactionalStore struct {
	tx          *sql.Tx
	categories  *CategoryRepository
	auditEvents *AuditEventRepository
}

func (store *categoryTransactionalStore) SaveCategory(ctx context.Context, categoryToSave category.Category) (*category.Category, error) {
	return store.categories.saveWithExecutor(ctx, store.tx, categoryToSave)
}

func (store *categoryTransactionalStore) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	return store.auditEvents.saveWithExecutor(ctx, store.tx, event)
}

func rollbackCategoryUnitOfWork(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back category unit of work: %v", cause, rollbackErr)
	}
	return cause
}
