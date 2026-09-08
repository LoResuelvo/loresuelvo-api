package evals

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetamorphicTransformsAreDeterministicAndDoNotMutateEvidence(t *testing.T) {
	input := RKInput{ProblemTitle: "problem", MaxResults: 3, Candidates: []CandidateInput{
		{Reference: "candidate-a", Evidence: EvidenceInput{RatingDistribution: []int{0, 0, 0, 0, 2}, WorkHistory: []WorkInput{{ID: 1, Description: "one", Review: &ReviewInput{Rating: 5}}, {ID: 2, Description: "two"}, {ID: 3, Description: "three"}}}},
		{Reference: "candidate-b", Evidence: EvidenceInput{WorkHistory: []WorkInput{{ID: 4}, {ID: 5}, {ID: 6}}}},
		{Reference: "candidate-c"},
	}}
	original, err := json.Marshal(input)
	require.NoError(t, err)
	for _, kind := range []string{CandidatePermutation, OpaqueReferenceBijection, WorkHistoryOrder} {
		t.Run(kind, func(t *testing.T) {
			first, err := TransformRanking(input, kind, 601)
			require.NoError(t, err)
			second, err := TransformRanking(input, kind, 601)
			require.NoError(t, err)
			require.Equal(t, first, second)
			actual, err := json.Marshal(input)
			require.NoError(t, err)
			require.Equal(t, original, actual)
			require.Equal(t, input.ProblemTitle, first.Input.ProblemTitle)
			require.Equal(t, input.MaxResults, first.Input.MaxResults)
			for _, candidate := range first.Input.Candidates {
				ref := candidate.Reference
				if len(first.InverseReferences) > 0 {
					ref = first.InverseReferences[ref]
				}
				var source CandidateInput
				for _, c := range input.Candidates {
					if c.Reference == ref {
						source = c
					}
				}
				require.ElementsMatch(t, source.Evidence.WorkHistory, candidate.Evidence.WorkHistory)
				require.Equal(t, source.Evidence.RatingDistribution, candidate.Evidence.RatingDistribution)
			}
			first.Input.Candidates[0].Reference = "mutated"
			require.Equal(t, "candidate-a", input.Candidates[0].Reference)
		})
	}
}

func TestMetamorphicNoOpAndInvalidInputs(t *testing.T) {
	for _, kind := range []string{CandidatePermutation, WorkHistoryOrder} {
		result, err := TransformRanking(RKInput{}, kind, 601)
		require.NoError(t, err)
		require.False(t, result.Changed)
	}
	_, err := TransformRanking(RKInput{}, "unsupported", 601)
	require.Error(t, err)
	_, err = TransformRanking(RKInput{Candidates: []CandidateInput{{Reference: "same"}, {Reference: "same"}}}, CandidatePermutation, 601)
	require.Error(t, err)
	result, err := TransformRanking(RKInput{Candidates: []CandidateInput{{Reference: "a"}}}, WorkHistoryOrder, 601)
	require.NoError(t, err)
	require.False(t, result.Changed)
}

func TestMetamorphicBijectionRoundTripRetainsNonReferenceFields(t *testing.T) {
	transformed, err := TransformRanking(RKInput{Candidates: []CandidateInput{{Reference: "a"}, {Reference: "b"}}}, OpaqueReferenceBijection, 601)
	require.NoError(t, err)
	require.True(t, transformed.Changed)
	ref := transformed.Input.Candidates[0].Reference
	raw, err := json.Marshal(map[string]any{"extra": true, "recommendations": []map[string]any{{"reference": ref, "reason": "literal " + ref, "extra": 42}}})
	require.NoError(t, err)
	restored, err := RestoreRankingOutput(raw, transformed.InverseReferences)
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, json.Unmarshal(restored, &output))
	require.Equal(t, true, output["extra"])
	item := output["recommendations"].([]any)[0].(map[string]any)
	require.Equal(t, "a", item["reference"])
	require.Equal(t, "literal "+ref, item["reason"])
	require.Equal(t, float64(42), item["extra"])
	_, err = RestoreRankingOutput(json.RawMessage(`{"recommendations":[{"reference":"unknown"}]}`), transformed.InverseReferences)
	require.Error(t, err)
	duplicate, err := json.Marshal(map[string]any{"recommendations": []map[string]string{{"reference": ref}, {"reference": ref}}})
	require.NoError(t, err)
	_, err = RestoreRankingOutput(duplicate, transformed.InverseReferences)
	require.Error(t, err)
	_, err = RestoreRankingOutput(raw, map[string]string{"x": "a", "y": "a"})
	require.Error(t, err)
}

func TestMetamorphicFrozenConfigSelectionRespectsParentSplitAndTrials(t *testing.T) {
	dataset, err := LoadDataset("../../evals/datasets/LoResuelvo_US60_evals_v1.0.0")
	require.NoError(t, err)
	require.Len(t, dataset.Metamorphic.Ranking, 24)
	require.Len(t, dataset.Metamorphic.PrediagnosisPairs, 2)
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "development", Model: "model", Trials: 3, MaxRequests: 1000})
	require.NoError(t, err)
	variants, err := BuildMetamorphicVariants(dataset, plan.Cases)
	require.NoError(t, err)
	require.Len(t, variants, 18*3*3)
	for _, v := range variants {
		require.Equal(t, "development", v.Split)
		require.Equal(t, 3, v.RepeatTrials)
		require.Equal(t, VariantID(v.BaseCaseID, v.Transformation, v.Seed), v.ID)
		require.Equal(t, "base_case_not_variant", v.AggregationUnit)
	}
	again, err := BuildMetamorphicVariants(dataset, plan.Cases)
	require.NoError(t, err)
	require.Equal(t, variants, again)
	dataset.Metamorphic.Ranking[0].Split = "holdout"
	require.Error(t, ValidateMetamorphicConfig(dataset))
}

func TestMetamorphicConfigRejectsCrossSplitPDPairs(t *testing.T) {
	dataset := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "left", FamilyID: "family", Split: "development"}}, {CaseMetadata: CaseMetadata{ID: "right", FamilyID: "family", Split: "holdout"}}}, Metamorphic: MetamorphicConfig{PrediagnosisPairs: []PDPair{{Left: "left", Right: "right", FamilyID: "family", Relation: "same_decision_despite_spelling"}}}}
	require.Error(t, ValidateMetamorphicConfig(dataset))
}

func TestMetamorphicPermutationPinsVersionedOrderingAndNoOpSeed(t *testing.T) {
	input := RKInput{Candidates: []CandidateInput{{Reference: "a"}, {Reference: "b"}, {Reference: "c"}}}
	changed, err := TransformRanking(input, CandidatePermutation, 601)
	require.NoError(t, err)
	require.True(t, changed.Changed)
	require.Equal(t, []CandidateInput{{Reference: "a"}, {Reference: "c"}, {Reference: "b"}}, changed.Input.Candidates)
	unchanged, err := TransformRanking(input, CandidatePermutation, 602)
	require.NoError(t, err)
	require.False(t, unchanged.Changed)
	require.Equal(t, input.Candidates, unchanged.Input.Candidates)
}
