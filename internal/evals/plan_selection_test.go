package evals

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPlanExplicitSelectionDoesNotExpandHoldout(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-dev", FamilyID: "dev", Split: "development"}}, {CaseMetadata: CaseMetadata{ID: "RK-reserve", FamilyID: "reserve", Split: "holdout"}}}, Suites: map[string][]string{"development": {"RK-dev"}, "holdout": {"RK-reserve"}}}
	for _, c := range dataset.RK {
		dataset.Metamorphic.Ranking = append(dataset.Metamorphic.Ranking, RankingTransformationPlan{BaseCaseID: c.ID, FamilyID: c.FamilyID, Split: c.Split, Transformations: []string{OpaqueReferenceBijection}, Seeds: []int64{1}, RepeatTrials: 1, AggregationUnit: "base_case_not_variant"})
	}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "development", Model: "model", Trials: 1, MaxRequests: 2, Metamorphic: true, CaseIDs: []string{"RK-dev"}})
	require.NoError(t, err)
	require.Len(t, plan.Cases, 2)
	require.False(t, plan.UsesHoldout)
	for _, planned := range plan.Cases {
		require.Equal(t, "development", planned.Split)
		if planned.Variant != nil {
			require.Equal(t, "RK-dev", planned.Variant.BaseCaseID)
		}
	}
	_, err = BuildPlan(dataset, PlanOptions{Suite: "development", Model: "model", Trials: 1, MaxRequests: 2, CaseIDs: []string{"RK-reserve"}})
	require.Error(t, err)
}
func TestPlanVariantBudgetIncludesTrialsAndRetries(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-dev", FamilyID: "family", Split: "development"}}}, Suites: map[string][]string{"development": {"RK-dev"}}, Metamorphic: MetamorphicConfig{Ranking: []RankingTransformationPlan{{BaseCaseID: "RK-dev", FamilyID: "family", Split: "development", Transformations: []string{CandidatePermutation, OpaqueReferenceBijection}, Seeds: []int64{1, 2}, RepeatTrials: 3, AggregationUnit: "base_case_not_variant"}}}}
	options := PlanOptions{Suite: "development", Model: "model", Trials: 3, MaxRetries: 1, MaxRequests: 30, Metamorphic: true}
	plan, err := BuildPlan(dataset, options)
	require.NoError(t, err)
	require.Equal(t, 3, plan.BaseExecutions)
	require.Equal(t, 12, plan.VariantExecutions)
	require.Equal(t, 30, plan.MaximumRequests)
	options.MaxRequests = 29
	_, err = BuildPlan(dataset, options)
	require.ErrorContains(t, err, "exceeding limit")
}
func TestPlanSelectionRejectsDuplicateAndUnknownCases(t *testing.T) {
	dataset := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-test", Split: "development"}}}, Suites: map[string][]string{"smoke": {"PD-test"}}}
	for _, ids := range [][]string{{"PD-test", "PD-test"}, {"unknown"}} {
		_, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "model", Trials: 1, MaxRequests: 2, CaseIDs: ids})
		require.Error(t, err)
	}
}
