package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type ProviderRegistrationUnitOfWork struct {
	db         *sql.DB
	users      *UserRepository
	categories *CategoryRepository
}

func NewProviderRegistrationUnitOfWork(db *sql.DB, users *UserRepository, categories *CategoryRepository) *ProviderRegistrationUnitOfWork {
	return &ProviderRegistrationUnitOfWork{db: db, users: users, categories: categories}
}
func (unit *ProviderRegistrationUnitOfWork) Execute(ctx context.Context, operation func(provider.RegistrationStore) error) error {
	tx, err := unit.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning provider registration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := operation(&providerRegistrationStore{tx: tx, users: unit.users, categories: unit.categories}); err != nil {
		return rollbackUserTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing provider registration: %w", err)
	}
	return nil
}

type providerRegistrationStore struct {
	tx         *sql.Tx
	users      *UserRepository
	categories *CategoryRepository
}

func (store *providerRegistrationStore) FindCategory(ctx context.Context, id int) (*category.Category, error) {
	return store.categories.findWithExecutor(ctx, store.tx, id, " FOR SHARE")
}
func (store *providerRegistrationStore) SaveUser(ctx context.Context, user user.User) (user.User, error) {
	return store.users.saveWithTx(ctx, store.tx, user)
}
