package provider

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
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

type RatingStatsReader interface {
	FindRatingStatsByProviderID(ctx context.Context, providerID int) (RatingStats, error)
}

type RatingStatsBatchReader interface {
	FindRatingStatsByProviderIDs(ctx context.Context, providerIDs []int) (map[int]RatingStats, error)
}

type PaidWorkHistoryReader interface {
	FindPaidWorkHistoryByProviderID(ctx context.Context, providerID int) ([]readmodel.WorkOrder, error)
}

type ProfileReaders struct {
	RatingStatsReader      RatingStatsReader
	RatingStatsBatchReader RatingStatsBatchReader
	PaidWorkHistoryReader  PaidWorkHistoryReader
}
