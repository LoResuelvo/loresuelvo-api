package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
)

type JobRequestCreationUnitOfWork struct {
	db         *sql.DB
	requests   *JobRequestRepository
	categories *CategoryRepository
	users      *UserRepository
}

func NewJobRequestCreationUnitOfWork(db *sql.DB, requests *JobRequestRepository, categories *CategoryRepository, users *UserRepository) *JobRequestCreationUnitOfWork {
	return &JobRequestCreationUnitOfWork{db: db, requests: requests, categories: categories, users: users}
}
func (unit *JobRequestCreationUnitOfWork) Execute(ctx context.Context, operation func(jobrequest.CreationStore) error) error {
	tx, err := unit.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning job request creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := operation(&jobRequestCreationStore{tx: tx, requests: unit.requests, categories: unit.categories, users: unit.users}); err != nil {
		return rollbackJobRequestTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing job request creation: %w", err)
	}
	return nil
}

type jobRequestCreationStore struct {
	tx         *sql.Tx
	requests   *JobRequestRepository
	categories *CategoryRepository
	users      *UserRepository
}

func (store *jobRequestCreationStore) FindProviderCategory(ctx context.Context, providerID int) (*category.Category, error) {
	categoryID, err := store.users.findProviderCategoryIDWithExecutor(ctx, store.tx, providerID)
	if err != nil {
		return nil, err
	}
	return store.categories.findWithExecutor(ctx, store.tx, categoryID, " FOR SHARE")
}
func (store *jobRequestCreationStore) SaveWithConversation(ctx context.Context, request jobrequest.JobRequest, pending conversation.Conversation) (*jobrequest.JobRequest, error) {
	work, ok := pending.(*conversation.WorkConversation)
	if !ok {
		return nil, fmt.Errorf("saving job request: expected work conversation")
	}
	return store.requests.saveWithConversationWithTx(ctx, store.tx, request, work)
}
