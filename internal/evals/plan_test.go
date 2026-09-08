package evals

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func planDataset() *Dataset {
	return &Dataset{Version: "test", ManifestSHA256: "hash", PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-001", Split: "development"}}, {CaseMetadata: CaseMetadata{ID: "PD-002", Split: "holdout"}}}, Suites: map[string][]string{"smoke": {"PD-001"}, "development": {"PD-001"}, "holdout": {"PD-002"}, "critical_all": {"PD-001", "PD-002"}}}
}
func TestBuildPlanCountsRetries(t *testing.T) {
	plan, err := BuildPlan(planDataset(), PlanOptions{Suite: "development", Model: "requested", Trials: 3, MaxRetries: 2, MaxRequests: 9})
	require.NoError(t, err)
	require.Equal(t, 3, plan.BaseExecutions)
	require.Equal(t, 9, plan.MaximumRequests)
	require.Zero(t, plan.LiveCalls)
	require.Nil(t, plan.Cost)
	require.False(t, plan.UsesHoldout)
}
func TestBuildPlanRequiresHoldoutAuthorization(t *testing.T) {
	for _, suite := range []string{"holdout", "critical_all", "smoke"} {
		t.Run(suite, func(t *testing.T) {
			d := planDataset()
			if suite == "smoke" {
				d.Suites[suite] = []string{"PD-002"}
			}
			options := PlanOptions{Suite: suite, Model: "requested", Trials: 1, MaxRequests: 2}
			_, err := BuildPlan(d, options)
			require.ErrorContains(t, err, "--allow-holdout")
			options.AllowHoldout = true
			plan, err := BuildPlan(d, options)
			require.NoError(t, err)
			require.True(t, plan.UsesHoldout)
		})
	}
}
func TestBuildPlanRejectsInvalidLimits(t *testing.T) {
	for _, options := range []PlanOptions{
		{Suite: "all", Model: "m", Trials: 1, MaxRequests: 10},
		{Suite: "smoke", Trials: 1, MaxRequests: 10},
		{Suite: "smoke", Model: "m", Trials: 0, MaxRequests: 10},
		{Suite: "smoke", Model: "m", Trials: 1, MaxRequests: 0},
		{Suite: "smoke", Model: "m", Trials: 1, MaxRetries: -1, MaxRequests: 10},
		{Suite: "smoke", Model: "m", Trials: 3, MaxRetries: 1, MaxRequests: 5},
		{Suite: "smoke", Model: "m", Trials: 1, MaxRetries: math.MaxInt, MaxRequests: math.MaxInt},
		{Suite: "smoke", Model: "m", Trials: math.MaxInt, MaxRetries: 1, MaxRequests: math.MaxInt},
	} {
		_, err := BuildPlan(planDataset(), options)
		require.Error(t, err)
	}
}
func TestBuildPlanRejectsUnknownAndDuplicateCases(t *testing.T) {
	for _, ids := range [][]string{{"CT-01"}, {"PD-001", "PD-001"}} {
		d := planDataset()
		d.Suites["smoke"] = ids
		_, err := BuildPlan(d, PlanOptions{Suite: "smoke", Model: "m", Trials: 1, MaxRequests: 10})
		require.Error(t, err)
	}
}
