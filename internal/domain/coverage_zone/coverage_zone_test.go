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
