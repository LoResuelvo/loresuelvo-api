package coveragezone_test

import (
	"testing"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/stretchr/testify/require"
)

func TestNewCoverageZoneOwnsInternalGeographicIdentity(t *testing.T) {
	zone, err := coveragezone.New(1, " caba-commune-06 ", "  Comuna 6  ", coveragezone.KindCommune)

	require.NoError(t, err)
	require.Equal(t, 1, zone.MarketID)
	require.Equal(t, "CABA-COMMUNE-06", zone.Code)
	require.Equal(t, "Comuna 6", zone.Name)
	require.Equal(t, "comuna 6", zone.NormalizedName)
	require.Equal(t, coveragezone.KindCommune, zone.Kind)
	require.True(t, zone.Enabled)
}

func TestNewCoverageZoneRequiresName(t *testing.T) {
	zone, err := coveragezone.New(1, "CABA-COMMUNE-06", "   ", coveragezone.KindCommune)

	require.ErrorIs(t, err, coveragezone.ErrNameRequired)
	require.Nil(t, zone)
}

func TestNewCoverageMarketOwnsStableInternalCode(t *testing.T) {
	market, err := coveragezone.NewMarket(" caba ", " Ciudad Autónoma de Buenos Aires ")

	require.NoError(t, err)
	require.Equal(t, "CABA", market.Code)
	require.Equal(t, "Ciudad Autónoma de Buenos Aires", market.Name)
	require.True(t, market.Enabled)
}

func TestCoverageZoneRejectsSelectionWhenDisabled(t *testing.T) {
	zone := coveragezone.CoverageZone{Enabled: false}

	err := zone.ValidateSelection()

	require.ErrorIs(t, err, coveragezone.ErrNotAvailable)
}

func TestCoverageZoneCanBeDisabled(t *testing.T) {
	zone := coveragezone.CoverageZone{Enabled: true}

	zone.Disable()

	require.False(t, zone.Enabled)
}

func TestCoverageZoneSelectionRejectsDuplicateIDs(t *testing.T) {
	err := coveragezone.ValidateUniqueIDs([]int{6, 14, 6})

	require.ErrorIs(t, err, coveragezone.ErrDuplicateCoverageZone)
}

func TestCoverageZoneSelectionAcceptsUniqueIDs(t *testing.T) {
	err := coveragezone.ValidateUniqueIDs([]int{6})

	require.NoError(t, err)
}
