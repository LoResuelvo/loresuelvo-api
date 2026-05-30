package provider

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
)

type Repository interface {
	Save(provider Provider) error
	FindByEmail(email string) bool
	FindByCategoryID(categoryID int) ([]Provider, error)
}

type CategoryFinder interface {
	FindByID(id int) *category.Category
}

type ProviderFinder interface {
	FindByIDs(ctx context.Context, ids []int) ([]Provider, error)
}
