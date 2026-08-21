package provider

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
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

type CoverageZoneFinder interface {
	FindByID(ctx context.Context, id int) (*coveragezone.CoverageZone, error)
}

type ProviderProfileReader interface {
	FindRatingStatsByProviderID(ctx context.Context, providerID int) (RatingStats, error)
	FindRatingStatsByProviderIDs(ctx context.Context, providerIDs []int) (map[int]RatingStats, error)
	FindPaidWorkHistoryByProviderID(ctx context.Context, providerID int) ([]readmodel.WorkOrder, error)
}
