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

func (repository *CategoryRepository) Save(category category.Category) error {
	_, err := repository.db.Exec(
		`INSERT INTO categories (name, normalized_name, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())`,
		category.Name,
		category.NormalizedName,
	)

	return err
}

func (repository *CategoryRepository) FindByNormalizedName(normalizedName string) bool {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM categories WHERE normalized_name = $1)`,
		normalizedName,
	).Scan(&exists)

	if err != nil {
		return false
	}

	return exists
}

func (repository *CategoryRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM categories`)
	return err
}
