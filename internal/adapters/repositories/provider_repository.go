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
	tx, err := repository.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var userID int
	err = tx.QueryRow(
		`INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id`,
		provider.User.AuthID,
		provider.User.Email,
		provider.User.Name,
		provider.User.Surname,
		provider.User.Role,
	).Scan(&userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`INSERT INTO providers (user_id, category_id)
		VALUES ($1, $2)`,
		userID,
		provider.Category.ID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (repository *ProviderRepository) DeleteAll() error {
	return repository.repositoryUser.DeleteAllOf("provider")
}

func (repository *ProviderRepository) FindByEmail(email string) bool {
	return repository.repositoryUser.FindByEmail(email)
}
