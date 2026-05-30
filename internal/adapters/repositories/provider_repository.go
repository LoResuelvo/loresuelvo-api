package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
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
		return fmt.Errorf("beginning provider transaction: %w", err)
	}

	userID, err := repository.repositoryUser.saveWithTx(tx, *provider.User)
	if err != nil {
		return rollbackProviderTx(tx, fmt.Errorf("saving provider user: %w", err))
	}

	_, err = tx.Exec(
		`INSERT INTO providers (user_id, category_id)
		VALUES ($1, $2)`,
		userID,
		provider.Category.ID,
	)
	if err != nil {
		return rollbackProviderTx(tx, fmt.Errorf("saving provider profile: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing provider transaction: %w", err)
	}

	return nil
}

func (repository *ProviderRepository) DeleteAll() error {
	return repository.repositoryUser.DeleteAllOf("provider")
}

func (repository *ProviderRepository) FindByEmail(email string) bool {
	return repository.repositoryUser.FindByEmail(email)
}

func (repository *ProviderRepository) ExistsByID(id int) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM providers WHERE id = $1)`,
		id,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking provider existence by id: %w", err)
	}

	return exists, nil
}

func (repository *ProviderRepository) FindIDByAuthID(authID string) (int, error) {
	var providerID int
	err := repository.db.QueryRow(
		`SELECT providers.id
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE users.auth_id = $1`,
		authID,
	).Scan(&providerID)
	if err != nil {
		return 0, fmt.Errorf("finding provider id by auth id: %w", err)
	}

	return providerID, nil
}

func (repository *ProviderRepository) FindIDByEmail(email string) (int, error) {
	var providerID int
	err := repository.db.QueryRow(
		`SELECT providers.id
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE users.email = $1`,
		email,
	).Scan(&providerID)
	if err != nil {
		return 0, fmt.Errorf("finding provider id by email: %w", err)
	}

	return providerID, nil
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

func (repository *ProviderRepository) FindByIDs(ctx context.Context, ids []int) ([]provider.Provider, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT
			providers.id,
			users.auth_id,
			users.email,
			users.name,
			users.surname,
			users.role,
			COALESCE(categories.name, ''),
			categories.id
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		LEFT JOIN categories ON categories.id = providers.category_id
		WHERE providers.id = ANY($1)
		ORDER BY users.name ASC, users.surname ASC`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("finding providers by ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []provider.Provider
	for rows.Next() {
		var prov provider.Provider
		var user user.User
		var categoryName string
		var categoryID sql.NullInt64

		if err := rows.Scan(&prov.ID, &user.AuthID, &user.Email, &user.Name, &user.Surname, &user.Role, &categoryName, &categoryID); err != nil {
			return nil, fmt.Errorf("scanning provider: %w", err)
		}

		prov.User = &user
		if categoryID.Valid {
			prov.Category = &category.Category{ID: int(categoryID.Int64), Name: categoryName}
		}

		results = append(results, prov)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating providers: %w", err)
	}

	return results, nil
}

func rollbackProviderTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback provider transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}
