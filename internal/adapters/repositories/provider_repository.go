package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
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

func (repository *ProviderRepository) FindByCategoryID(categoryID int) ([]provider.Provider, error) {
	rows, err := repository.db.Query(
		`SELECT providers.id, users.auth_id, users.email, users.name, users.surname, users.role
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE providers.category_id = $1
		ORDER BY users.name ASC, users.surname ASC`,
		categoryID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	providers := []provider.Provider{}
	for rows.Next() {
		var providerID int
		var providerUser user.User
		if err := rows.Scan(
			&providerID,
			&providerUser.AuthID,
			&providerUser.Email,
			&providerUser.Name,
			&providerUser.Surname,
			&providerUser.Role,
		); err != nil {
			return nil, err
		}

		providers = append(providers, provider.Provider{ID: providerID, User: &providerUser})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return providers, nil
}
