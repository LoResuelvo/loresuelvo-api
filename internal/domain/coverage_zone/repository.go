package coveragezone

import "context"

type Repository interface {
	ListAvailableByMarketCode(ctx context.Context, marketCode string) ([]CatalogEntry, error)
}
