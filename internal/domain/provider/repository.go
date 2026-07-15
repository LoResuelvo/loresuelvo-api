package provider

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository interface {
	Save(ctx context.Context, user user.User) (user.User, error)
	FindByEmail(email string) bool
	FindProviderByID(ctx context.Context, providerID int) (*Provider, error)
	FindProvidersByCategoryID(categoryID int) ([]Provider, error)
}

type CategoryFinder interface {
	FindByID(id int) *category.Category
}
