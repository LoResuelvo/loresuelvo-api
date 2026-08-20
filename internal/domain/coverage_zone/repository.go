package coveragezone

import "context"

type Repository interface {
	Save(ctx context.Context, zone CoverageZone) (*CoverageZone, error)
	FindByName(ctx context.Context, name string) (*CoverageZone, error)
	FindByProviderID(ctx context.Context, providerID int) ([]CoverageZone, error)
	DeleteAll() error
}
