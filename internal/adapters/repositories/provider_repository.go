package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type ProviderRepository struct {
	db             *sql.DB
	repositoryUser *UserRepository
}

func NewProviderRepository(db *sql.DB, repositoryUser *UserRepository) *ProviderRepository {
	return &ProviderRepository{
		db:             db,
		repositoryUser: repositoryUser,
	}
}

func (repository *ProviderRepository) Save(provider provider.Provider) error {
	return repository.repositoryUser.Save(*provider.User)
}

func (repository *ProviderRepository) DeleteAll() error {
	return repository.repositoryUser.DeleteAllOf("provider")
}

func (repository *ProviderRepository) FindByEmail(email string) bool {
	return repository.repositoryUser.FindByEmail(email)
}
