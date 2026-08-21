package coveragezone

import "context"

type Repository interface {
	Save(ctx context.Context, zone CoverageZone) (*CoverageZone, error)
	FindByName(ctx context.Context, name string) (*CoverageZone, error)
	FindByID(ctx context.Context, id int) (*CoverageZone, error)
	FindByProviderID(ctx context.Context, providerID int) ([]CoverageZone, error)
	Update(ctx context.Context, zone CoverageZone) error
	DeleteAll() error
}
