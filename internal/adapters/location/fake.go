package location

import (
	"context"
	"strings"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
)

const (
	fakeValidAddressStreet = "av. rivadavia"
	fakeOutsideAddress     = "av. maipú, vicente lópez"
	fakeInvalidAddress     = "domicilio inexistente"
)

type FakeAddressResolver struct {
	mu        sync.RWMutex
	available bool
}

func NewFakeAddressResolver() *FakeAddressResolver {
	return &FakeAddressResolver{available: true}
}

func (resolver *FakeAddressResolver) SetAvailable(available bool) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.available = available
}

func (resolver *FakeAddressResolver) Resolve(_ context.Context, address consumer.Address) (consumer.GeoPoint, error) {
	resolver.mu.RLock()
	available := resolver.available
	resolver.mu.RUnlock()
	if !available {
		return consumer.GeoPoint{}, consumer.ErrAddressServiceUnavailable
	}

	street := strings.ToLower(strings.TrimSpace(address.Street))
	switch street {
	case fakeInvalidAddress:
		return consumer.GeoPoint{}, consumer.ErrAddressNotValidated
	case fakeOutsideAddress:
		return consumer.NewGeoPoint(-34.528, -58.476)
	case fakeValidAddressStreet:
		return consumer.NewGeoPoint(-34.6208, -58.4386)
	default:
		return consumer.GeoPoint{}, consumer.ErrAddressNotValidated
	}
}

type FakeCoverageZoneResolver struct {
	zoneFinder CoverageZoneByMarketAndNameFinder
}

func NewFakeCoverageZoneResolver(zoneFinder CoverageZoneByMarketAndNameFinder) *FakeCoverageZoneResolver {
	return &FakeCoverageZoneResolver{zoneFinder: zoneFinder}
}

func (resolver *FakeCoverageZoneResolver) Resolve(ctx context.Context, point consumer.GeoPoint) (*coveragezone.CoverageZone, error) {
	if resolver == nil || resolver.zoneFinder == nil {
		return nil, consumer.ErrAddressServiceUnavailable
	}
	if point.Latitude > -34.55 {
		return nil, coveragezone.ErrDoesNotExist
	}

	return resolver.zoneFinder.FindByMarketCodeAndName(ctx, "CABA", "Comuna 6")
}
