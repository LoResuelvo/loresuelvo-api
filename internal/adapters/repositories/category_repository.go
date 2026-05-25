package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
)

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (repository *CategoryRepository) Save(categoryToSave category.Category) (*category.Category, error) {
	var savedCategory category.Category
	err := repository.db.QueryRow(
		`INSERT INTO categories (name, normalized_name, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, name, normalized_name`,
		categoryToSave.Name,
		categoryToSave.NormalizedName,
	).Scan(&savedCategory.ID, &savedCategory.Name, &savedCategory.NormalizedName)
	if err != nil {
		return nil, err
	}

	return &savedCategory, nil
}

func (repository *CategoryRepository) ListAll() ([]category.Category, error) {
	rows, err := repository.db.Query(
		`SELECT id, name, normalized_name FROM categories ORDER BY name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	categories := []category.Category{}
	for rows.Next() {
		var category category.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.NormalizedName); err != nil {
			return nil, err
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, err
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
