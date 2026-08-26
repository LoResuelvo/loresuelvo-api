package coveragezone_test

import (
	"context"
	"errors"
	"testing"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type coverageZoneRepositoryStub struct {
	zones      []coveragezone.CatalogEntry
	err        error
	marketCode string
}

func (stub *coverageZoneRepositoryStub) ListAvailableByMarketCode(_ context.Context, marketCode string) ([]coveragezone.CatalogEntry, error) {
	stub.marketCode = marketCode
	return stub.zones, stub.err
}

func TestCoverageZoneServiceListsAvailableZonesFromDefaultMarket(t *testing.T) {
	zone := coveragezone.CoverageZone{ID: 6, Code: "CABA-COMMUNE-06", Name: "Comuna 6", Enabled: true}
	entry := coveragezone.CatalogEntry{
		Zone: zone,
		BoundaryReference: coveragezone.ExternalReference{
			CoverageZoneID: zone.ID,
			Provider:       "GOOGLE",
			ExternalID:     "google-place-6",
		},
	}
	repository := &coverageZoneRepositoryStub{zones: []coveragezone.CatalogEntry{entry}}
	service := coveragezone.NewService(repository)

	entries, err := service.List(context.Background())

	require.NoError(t, err)
	assert.Equal(t, coveragezone.DefaultMarketCode, repository.marketCode)
	assert.Equal(t, []coveragezone.CatalogEntry{entry}, entries)
}

func TestCoverageZoneServiceNormalizesNilRepositoryResultToEmptyList(t *testing.T) {
	service := coveragezone.NewService(&coverageZoneRepositoryStub{})

	entries, err := service.List(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, entries)
	assert.Empty(t, entries)
}

func TestCoverageZoneServicePropagatesRepositoryFailure(t *testing.T) {
	expectedErr := errors.New("catalog database unavailable")
	service := coveragezone.NewService(&coverageZoneRepositoryStub{err: expectedErr})

	entries, err := service.List(context.Background())

	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, entries)
}
