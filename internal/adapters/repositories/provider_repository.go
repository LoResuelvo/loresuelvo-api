package repositories

import (
	"context"
	"database/sql"
	"errors"
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

func (repository *ProviderRepository) Save(providerToSave provider.Provider) (int, error) {
	tx, err := repository.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning provider transaction: %w", err)
	}

	userID, err := repository.repositoryUser.saveWithTx(tx, *providerToSave.User)
	if err != nil {
		return 0, rollbackProviderTx(tx, fmt.Errorf("saving provider user: %w", err))
	}

	var providerID int
	err = tx.QueryRow(
		`INSERT INTO providers (user_id, category_id, profile_photo_file_id)
		VALUES ($1, $2, $3)
		RETURNING id`,
		userID,
		providerToSave.Category.ID,
		providerToSave.ProfilePhotoFileID,
	).Scan(&providerID)
	if err != nil {
		return 0, rollbackProviderTx(tx, fmt.Errorf("saving provider profile: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing provider transaction: %w", err)
	}

	return providerID, nil
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

func (repository *ProviderRepository) FindByID(ctx context.Context, providerID int) (*provider.Provider, error) {
	row := repository.db.QueryRowContext(ctx, providerSelectSQL+` WHERE providers.id = $1`, providerID)
	foundProvider, err := scanProvider(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provider.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding provider by id: %w", err)
	}
	return foundProvider, nil
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

func (repository *ProviderRepository) FindAuthIDByID(providerID int) (string, error) {
	var authID string
	err := repository.db.QueryRow(
		`SELECT users.auth_id
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE providers.id = $1`,
		providerID,
	).Scan(&authID)
	if err != nil {
		return "", fmt.Errorf("finding provider auth id by id: %w", err)
	}

	return authID, nil
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
		providerSelectSQL+` WHERE providers.category_id = $1
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
		foundProvider, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, *foundProvider)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return providers, nil
}

const providerSelectSQL = `SELECT providers.id,
	users.auth_id,
	users.email,
	users.name,
	users.surname,
	users.role,
	categories.id,
	categories.name,
	categories.normalized_name,
	COALESCE(providers.profile_photo_file_id::text, '')
FROM providers
INNER JOIN users ON users.id = providers.user_id
INNER JOIN categories ON categories.id = providers.category_id`

type providerScanner interface {
	Scan(dest ...any) error
}

func scanProvider(scanner providerScanner) (*provider.Provider, error) {
	var foundProvider provider.Provider
	var providerCategory category.Category
	var providerUser user.User
	if err := scanner.Scan(
		&foundProvider.ID,
		&providerUser.AuthID,
		&providerUser.Email,
		&providerUser.Name,
		&providerUser.Surname,
		&providerUser.Role,
		&providerCategory.ID,
		&providerCategory.Name,
		&providerCategory.NormalizedName,
		&foundProvider.ProfilePhotoFileID,
	); err != nil {
		return nil, err
	}
	foundProvider.User = &providerUser
	foundProvider.Category = &providerCategory
	return &foundProvider, nil
}

func rollbackProviderTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback provider transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}
