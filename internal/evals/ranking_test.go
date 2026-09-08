package evals

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRankingNDCGExponentialGain(t *testing.T) {
	result := rankingNDCG([]string{"b", "a"}, map[string]int{"a": 3, "b": 2})
	require.NotNil(t, result)
	require.InDelta(t, 0.8339912323981488, *result, 0.0000001)
}
func TestRankingNDCGZeroIdealIsUnassessed(t *testing.T) {
	require.Nil(t, rankingNDCG([]string{"a"}, map[string]int{"a": 0}))
}
func TestRankingPairwiseReportsCoverage(t *testing.T) {
	result := rankingPairwise([]string{"a"}, []PairwiseConstraint{{Higher: "a", Lower: "b"}, {Higher: "b", Lower: "c"}})
	require.Equal(t, 1, result.Passed)
	require.Equal(t, 1, result.Unassessed)
	require.Equal(t, 0.5, *result.Coverage)
	require.Equal(t, 1.0, *result.Satisfaction)
}
func TestRankingPairwiseMissingHigherFails(t *testing.T) {
	result := rankingPairwise([]string{"b"}, []PairwiseConstraint{{Higher: "a", Lower: "b"}})
	require.Equal(t, 1, result.Failed)
}
