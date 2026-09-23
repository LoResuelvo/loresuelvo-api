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

type categoryInsertExecutor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (repository *CategoryRepository) saveWithExecutor(ctx context.Context, executor categoryInsertExecutor, categoryToSave category.Category) (*category.Category, error) {
	var savedCategory category.Category
	err := executor.QueryRowContext(ctx,
		`INSERT INTO categories (name, normalized_name, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, name, normalized_name`,
		categoryToSave.Name,
		categoryToSave.NormalizedName,
	).Scan(&savedCategory.ID, &savedCategory.Name, &savedCategory.NormalizedName)
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
		`SELECT id, name, normalized_name FROM categories ORDER BY name ASC`,
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
		if err := rows.Scan(&category.ID, &category.Name, &category.NormalizedName); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating categories: %w", err)
	}

	return categories, nil
}

func (repository *CategoryRepository) FindByNormalizedName(normalizedName string) *category.Category {
	var category category.Category
	err := repository.db.QueryRow(
		`SELECT id, name, normalized_name FROM categories WHERE normalized_name = $1`,
		normalizedName,
	).Scan(&category.ID, &category.Name, &category.NormalizedName)

	if err != nil {
		return nil
	}

	return &category
}

func (repository *CategoryRepository) FindByID(id int) *category.Category {
	var category category.Category
	err := repository.db.QueryRow(
		`SELECT id, name, normalized_name FROM categories WHERE id = $1`,
		id,
	).Scan(&category.ID, &category.Name, &category.NormalizedName)

	if err != nil {
		return nil
	}

	return &category
}

func (repository *CategoryRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM categories`)
	return err
}
