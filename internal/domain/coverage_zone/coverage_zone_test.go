package coveragezone_test

import (
	"testing"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/stretchr/testify/require"
)

func TestNewCoverageZoneTrimsNameAndEnablesZone(t *testing.T) {
	zone, err := coveragezone.New("  Comuna 6  ")

	require.NoError(t, err)
	require.Equal(t, "Comuna 6", zone.Name)
	require.Equal(t, "comuna 6", zone.NormalizedName)
	require.True(t, zone.Enabled)
}

func TestNewCoverageZoneRequiresName(t *testing.T) {
	zone, err := coveragezone.New("   ")

	require.ErrorIs(t, err, coveragezone.ErrNameRequired)
	require.Nil(t, zone)
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
