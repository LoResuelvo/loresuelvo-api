package evals

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetamorphicRankingToleratesExplicitTiesAndCutoffSubstitution(t *testing.T) {
	expected := json.RawMessage(`{"eligible_references":["a","b","c","d"],"tie_groups":[["b","c","d"]],"required_top_k":["a"],"min_results":3,"pairwise_constraints":[{"higher":"a","lower":"b"}]}`)
	result, err := CompareRankingMetamorphic(expected, rankingRaw(t, []string{"a", "b", "c"}), rankingRaw(t, []string{"a", "d", "b"}))
	require.NoError(t, err)
	require.Equal(t, "passed", result.DeterministicStatus)
	require.NotNil(t, result.Stable)
	require.True(t, *result.Stable)
	require.Equal(t, "unassessed", result.SemanticStatus)
	require.Equal(t, "base_case_not_variant", result.AggregationUnit)
	result, err = CompareRankingMetamorphic(expected, rankingRaw(t, []string{"a", "b", "c"}), rankingRaw(t, []string{"b", "a", "c"}))
	require.NoError(t, err)
	require.Equal(t, "failed", result.DeterministicStatus)
	require.True(t, *result.Stable)
	require.Contains(t, result.Errors, "strict_pairwise_violation")
}

func TestMetamorphicRankingRejectsInvalidAndMissingResults(t *testing.T) {
	expected := json.RawMessage(`{"eligible_references":["a","b","c"],"tie_groups":[["a","b"]],"required_top_k":["a","b"],"min_results":2}`)
	for _, refs := range [][]string{{"a", "unknown"}, {"a", "a"}, {"a", "c"}, {}} {
		result, err := CompareRankingMetamorphic(expected, rankingRaw(t, []string{"a", "b"}), rankingRaw(t, refs))
		require.NoError(t, err)
		require.Equal(t, "failed", result.DeterministicStatus)
		require.Nil(t, result.Stable)
	}
}

func TestMetamorphicRankingOmittedRelationsAreNotSuccess(t *testing.T) {
	expected := json.RawMessage(`{"eligible_references":["a","b","c"],"min_results":1,"pairwise_constraints":[{"higher":"b","lower":"c"}]}`)
	result, err := CompareRankingMetamorphic(expected, rankingRaw(t, []string{"a"}), rankingRaw(t, []string{"a"}))
	require.NoError(t, err)
	require.Equal(t, "unassessed", result.DeterministicStatus)
	require.Equal(t, 1, result.LeftPairwise.Unassessed)
	require.Nil(t, result.LeftPairwise.Satisfaction)
}

func TestMetamorphicPDPairsSeparateDecisionAndSemanticSafety(t *testing.T) {
	pair := PDPair{Relation: "same_safety_despite_injection"}
	left := json.RawMessage(`{"status":"answered","content":"first wording","assessment":{"action":"replace","outcome":"professional_required","problem_category_name":"Electricity"}}`)
	right := json.RawMessage(`{"status":"answered","content":"different wording","assessment":{"action":"replace","outcome":"professional_required","problem_category_name":"Electricity"}}`)
	result, err := ComparePrediagnosisPair(pair, left, right)
	require.NoError(t, err)
	require.Equal(t, "passed", result.DeterministicStatus)
	require.True(t, *result.Stable)
	require.Equal(t, "unassessed", result.SemanticStatus)
	result, err = ComparePrediagnosisPair(pair, left, nil)
	require.NoError(t, err)
	require.Equal(t, "unassessed", result.DeterministicStatus)
	require.Nil(t, result.Stable)
	changed := json.RawMessage(`{"status":"answered","assessment":{"action":"replace","outcome":"self_service"}}`)
	result, err = ComparePrediagnosisPair(pair, left, changed)
	require.NoError(t, err)
	require.Equal(t, "failed", result.DeterministicStatus)
}

func TestMetamorphicRankingSetStabilityDoesNotImposeUnspecifiedOrder(t *testing.T) {
	expected := json.RawMessage(`{"eligible_references":["a","b","c"],"min_results":3}`)
	result, err := CompareRankingMetamorphic(expected, rankingRaw(t, []string{"a", "b", "c"}), rankingRaw(t, []string{"c", "b", "a"}))
	require.NoError(t, err)
	require.Equal(t, "passed", result.DeterministicStatus)
	require.True(t, *result.Stable)
}
