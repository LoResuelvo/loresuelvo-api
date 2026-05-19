package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type ProviderRepository struct {
	db *sql.DB
}

func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{
		db: db,
	}
}

func (repository *ProviderRepository) Save(provider provider.Provider) error {
	_, err := repository.db.Exec(
		`INSERT INTO providers (auth0_id, email, name, surname, created_on, updated_on)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		provider.Auth0ID,
		provider.Email,
		provider.Name,
		provider.Surname,
	)

	return err
}

func (repository *ProviderRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM providers`)
	return err
}

func (repository *ProviderRepository) FindByEmail(email string) bool {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM providers WHERE email = $1)`,
		email,
	).Scan(&exists)

	if err != nil {
		return false
	}

	return exists
}
