package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository struct {
	db *sql.DB
}

const userWithProfileSelectSQL = `SELECT u.id, u.auth_id, u.email, u.name, u.surname, u.role,
	c.user_id, p.user_id, cat.id, cat.name, cat.normalized_name,
	COALESCE(p.profile_photo_file_id::text, '')
	FROM users u
	LEFT JOIN consumers c ON c.user_id = u.id
	LEFT JOIN providers p ON p.user_id = u.id
	LEFT JOIN categories cat ON cat.id = p.category_id`

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (repository *UserRepository) Save(ctx context.Context, userToSave user.User) (user.User, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning user transaction: %w", err)
	}

	userID, err := saveBaseUser(ctx, tx, userToSave.Base())
	if err != nil {
		return nil, rollbackUserTx(tx, err)
	}
	userToSave.Base().ID = userID

	switch typedUser := userToSave.(type) {
	case *consumer.Consumer:
		if typedUser.Role != consumer.Role {
			err = fmt.Errorf("consumer has role %q", typedUser.Role)
			break
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO consumers (user_id) VALUES ($1)`, typedUser.ID)
	case *provider.Provider:
		if typedUser.Role != provider.Role || typedUser.Category == nil {
			err = fmt.Errorf("provider has invalid role or category")
			break
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO providers (user_id, category_id, profile_photo_file_id) VALUES ($1, $2, $3)`,
			typedUser.ID, typedUser.Category.ID, typedUser.ProfilePhotoFileID)
	default:
		err = fmt.Errorf("saving user: unsupported user type %T", userToSave)
	}
	if err != nil {
		return nil, rollbackUserTx(tx, fmt.Errorf("saving user profile: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing user transaction: %w", err)
	}
	return userToSave, nil
}

type userQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func saveBaseUser(ctx context.Context, queryRower userQueryRower, userToSave *user.BaseUser) (int, error) {
	var userID int
	err := queryRower.QueryRowContext(
		ctx,
		`INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id`,
		userToSave.AuthID,
		userToSave.Email,
		userToSave.Name,
		userToSave.Surname,
		userToSave.Role,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("saving user: %w", err)
	}

	return userID, nil
}

func (repository *UserRepository) FindByEmail(email string) bool {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`,
		email,
	).Scan(&exists)

	if err != nil {
		return false
	}

	return exists
}

func (repository *UserRepository) FindByAuthID(authID string) (user.User, error) {
	return scanUserWithProfile(
		repository.db.QueryRow(userWithProfileSelectSQL+" WHERE u.auth_id = $1", authID),
		"auth id",
	)
}

func (repository *UserRepository) FindByID(ctx context.Context, id int) (user.User, error) {
	return scanUserWithProfile(
		repository.db.QueryRowContext(ctx, userWithProfileSelectSQL+" WHERE u.id = $1", id),
		"id",
	)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserWithProfile(row rowScanner, lookup string) (user.User, error) {
	var base user.BaseUser
	var consumerID, providerID, categoryID sql.NullInt64
	var categoryName, normalizedCategoryName, profilePhotoFileID sql.NullString
	err := row.Scan(
		&base.ID, &base.AuthID, &base.Email, &base.Name, &base.Surname, &base.Role,
		&consumerID, &providerID, &categoryID, &categoryName, &normalizedCategoryName, &profilePhotoFileID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding user by %s: %w", lookup, err)
	}
	switch base.Role {
	case consumer.Role:
		if !consumerID.Valid {
			return nil, fmt.Errorf("finding user by %s: consumer profile is missing", lookup)
		}
		return &consumer.Consumer{BaseUser: &base}, nil
	case provider.Role:
		if !providerID.Valid || !categoryID.Valid {
			return nil, fmt.Errorf("finding user by %s: provider profile is incomplete", lookup)
		}
		return &provider.Provider{
			BaseUser: &base,
			Category: &category.Category{
				ID:             int(categoryID.Int64),
				Name:           categoryName.String,
				NormalizedName: normalizedCategoryName.String,
			},
			ProfilePhotoFileID: profilePhotoFileID.String,
		}, nil
	default:
		return nil, fmt.Errorf("finding user by %s: unsupported role %q", lookup, base.Role)
	}
}

func (repository *UserRepository) FindIDByAuthID(authID string) (int, error) {
	foundUser, err := repository.FindByAuthID(authID)
	if err != nil {
		return 0, err
	}
	return foundUser.Base().ID, nil
}

func rollbackUserTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback user transaction: %v", originalErr, rollbackErr)
	}
	return originalErr
}

func (repository *UserRepository) FindAuthIDByID(id int) (string, error) {
	var authID string
	if err := repository.db.QueryRow(`SELECT auth_id FROM users WHERE id = $1`, id).Scan(&authID); err != nil {
		return "", fmt.Errorf("finding user auth id by id: %w", err)
	}
	return authID, nil
}

func (repository *UserRepository) FindIDByEmail(email string) (int, error) {
	var id int
	if err := repository.db.QueryRow(`SELECT id FROM users WHERE email = $1`, email).Scan(&id); err != nil {
		return 0, fmt.Errorf("finding user id by email: %w", err)
	}
	return id, nil
}

func (repository *UserRepository) FindConsumerByID(consumerID int) (*consumer.Consumer, error) {
	var foundConsumer consumer.Consumer
	var consumerUser user.BaseUser
	err := repository.db.QueryRow(
		`SELECT consumers.user_id,
			users.auth_id,
			users.email,
			users.name,
			users.surname,
			users.role
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE consumers.user_id = $1`,
		consumerID,
	).Scan(
		&consumerUser.ID,
		&consumerUser.AuthID,
		&consumerUser.Email,
		&consumerUser.Name,
		&consumerUser.Surname,
		&consumerUser.Role,
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumer by id: %w", err)
	}

	foundConsumer.BaseUser = &consumerUser
	return &foundConsumer, nil
}

func (repository *UserRepository) ExistsProviderByID(id int) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM providers WHERE user_id = $1)`,
		id,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking provider existence by id: %w", err)
	}

	return exists, nil
}

func (repository *UserRepository) FindProviderByID(ctx context.Context, providerID int) (*provider.Provider, error) {
	row := repository.db.QueryRowContext(ctx, providerSelectSQL+` WHERE providers.user_id = $1`, providerID)
	foundProvider, err := scanProvider(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provider.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding provider by id: %w", err)
	}
	return foundProvider, nil
}

func (repository *UserRepository) FindProviderByAuthID(authID string) (*provider.Provider, error) {
	foundUser, err := repository.FindByAuthID(authID)
	if err != nil {
		return nil, fmt.Errorf("finding provider by auth id: %w", err)
	}
	foundProvider, ok := foundUser.(*provider.Provider)
	if !ok {
		return nil, provider.ErrDoesNotExist
	}
	return foundProvider, nil
}

func (repository *UserRepository) FindProvidersByCategoryID(categoryID int) ([]provider.Provider, error) {
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

const providerSelectSQL = `SELECT providers.user_id,
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
	var providerUser user.BaseUser
	if err := scanner.Scan(
		&providerUser.ID,
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
	foundProvider.BaseUser = &providerUser
	foundProvider.Category = &providerCategory
	return &foundProvider, nil
}

func (repository *UserRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM users`)
	return err
}

func (repository *UserRepository) DeleteAllOf(role string) error {
	_, err := repository.db.Exec(`DELETE FROM users WHERE role = $1`, role)
	return err
}
