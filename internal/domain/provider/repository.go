package provider

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/category"

type Repository interface {
	Save(provider Provider) (int, error)
	FindByEmail(email string) bool
	FindByCategoryID(categoryID int) ([]Provider, error)
}

type CategoryFinder interface {
	FindByID(id int) *category.Category
}
