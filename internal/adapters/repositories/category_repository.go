package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/jackc/pgx/v5/pgconn"
)

const categoryNormalizedNameUniqueConstraint = "categories_normalized_name_unique"

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (repository *CategoryRepository) Save(categoryToSave category.Category) (*category.Category, error) {
	return repository.saveWithExecutor(context.Background(), repository.db, categoryToSave)
}

type categoryQueryExecutor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (repository *CategoryRepository) saveWithExecutor(ctx context.Context, executor categoryQueryExecutor, categoryToSave category.Category) (*category.Category, error) {
	var savedCategory category.Category
	query := `INSERT INTO categories (name, normalized_name, enabled, version, created_on, updated_on)
 VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id, name, normalized_name, enabled, version`
	args := []any{categoryToSave.Name, categoryToSave.NormalizedName, categoryToSave.Enabled, categoryToSave.Version}
	if categoryToSave.ID != 0 {
		query = `UPDATE categories SET name=$1, normalized_name=$2, enabled=$3, version=$4, updated_on=NOW() WHERE id=$5 RETURNING id,name,normalized_name,enabled,version`
		args = append(args, categoryToSave.ID)
	}
	err := executor.QueryRowContext(ctx, query, args...).Scan(&savedCategory.ID, &savedCategory.Name, &savedCategory.NormalizedName, &savedCategory.Enabled, &savedCategory.Version)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, category.ErrDoesNotExist
	}
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == categoryNormalizedNameUniqueConstraint {
			return nil, category.ErrAlreadyExists
		}

		return nil, fmt.Errorf("inserting category: %w", err)
	}

	return &savedCategory, nil
}

func (repository *CategoryRepository) ListAll() ([]category.Category, error) {
	rows, err := repository.db.Query(
		`SELECT id, name, normalized_name, enabled, version FROM categories ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	categories := []category.Category{}
	for rows.Next() {
		var category category.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.NormalizedName, &category.Enabled, &category.Version); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating categories: %w", err)
	}

	return categories, nil
}

func (repository *CategoryRepository) FindByNormalizedName(ctx context.Context, normalizedName string) (*category.Category, error) {
	var found category.Category
	err := repository.db.QueryRowContext(ctx, `SELECT id,name,normalized_name,enabled,version FROM categories WHERE normalized_name=$1`, normalizedName).Scan(&found.ID, &found.Name, &found.NormalizedName, &found.Enabled, &found.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, category.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding category by normalized name: %w", err)
	}
	return &found, nil
}

func (repository *CategoryRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM categories`)
	return err
}

func (repository *CategoryRepository) FindByID(ctx context.Context, id int) (*category.Category, error) {
	return repository.findWithExecutor(ctx, repository.db, id, "")
}

func (repository *CategoryRepository) findWithExecutor(ctx context.Context, executor categoryQueryExecutor, id int, lock string) (*category.Category, error) {
	var found category.Category
	err := executor.QueryRowContext(ctx, `SELECT id,name,normalized_name,enabled,version FROM categories WHERE id=$1`+lock, id).Scan(&found.ID, &found.Name, &found.NormalizedName, &found.Enabled, &found.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, category.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding category: %w", err)
	}
	return &found, nil
}
