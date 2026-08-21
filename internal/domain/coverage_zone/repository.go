package coveragezone

import "context"

type Repository interface {
	SaveMarket(ctx context.Context, market Market) (*Market, error)
	FindMarketByCode(ctx context.Context, code string) (*Market, error)
	Save(ctx context.Context, zone CoverageZone) (*CoverageZone, error)
	FindByMarketCodeAndName(ctx context.Context, marketCode, name string) (*CoverageZone, error)
	SaveExternalReference(ctx context.Context, reference ExternalReference) error
	FindByExternalReference(ctx context.Context, provider, externalID string) (*CoverageZone, error)
	FindByID(ctx context.Context, id int) (*CoverageZone, error)
	FindByProviderID(ctx context.Context, providerID int) ([]CoverageZone, error)
	Update(ctx context.Context, zone CoverageZone) error
	DeleteAll() error
}
