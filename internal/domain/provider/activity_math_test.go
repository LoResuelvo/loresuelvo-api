package provider

import (
	"math"
	"testing"

	read_model "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/require"
)

func TestActivityAverageRoundsHalfUpWithoutOverflow(t *testing.T) {
	result := activityResults(read_model.ActivityCounts{Reported: 2, ContractValueCents: 30003})
	require.Equal(t, int64(15002), *result.AverageCents)
	large := activityResults(read_model.ActivityCounts{Reported: 2, ContractValueCents: math.MaxInt64})
	require.Equal(t, int64(4611686018427387904), *large.AverageCents)
	require.Nil(t, activityResults(read_model.ActivityCounts{}).AverageCents)
}

func TestActivityPercentRoundsHalfAwayFromZero(t *testing.T) {
	require.Equal(t, 0.01, *percentChange(20001, 20000))
	require.Equal(t, -0.01, *percentChange(19999, 20000))
	require.Equal(t, -100.0, *percentChange(0, 1))
	require.Nil(t, percentChange(1, 0))
}
