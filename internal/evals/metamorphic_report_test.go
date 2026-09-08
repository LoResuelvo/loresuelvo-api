package evals

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func metamorphicReportFixture(t *testing.T, kind string, seed int64) (*Dataset, RunRecord, Attempt, Attempt) {
	t.Helper()
	dataset, record, base := replayEvidence(t)
	dataset.RK[0].FamilyID = "family"
	dataset.RK[0].Input = RKInput{ProblemTitle: "problem", MaxResults: 3, Candidates: []CandidateInput{{Reference: "a", Evidence: EvidenceInput{RatingDistribution: []int{0, 0, 0, 0, 0}}}, {Reference: "b", Evidence: EvidenceInput{RatingDistribution: []int{0, 0, 0, 0, 0}}}, {Reference: "c", Evidence: EvidenceInput{RatingDistribution: []int{0, 0, 0, 0, 0}}}}}
	dataset.Metamorphic = MetamorphicConfig{Ranking: []RankingTransformationPlan{{BaseCaseID: "RK-test", FamilyID: "family", Split: "development", Transformations: []string{kind}, Seeds: []int64{seed}, RepeatTrials: 1, AggregationUnit: "base_case_not_variant"}}}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "model-a", Trials: 1, MaxRequests: 2, Metamorphic: true})
	require.NoError(t, err)
	record.Plan = plan
	transformed, err := TransformRanking(dataset.RK[0].Input, kind, seed)
	require.NoError(t, err)
	variant := base
	variant.CaseID = plan.Cases[1].CaseID
	refs := []string{"a", "b", "c"}
	if len(transformed.InverseReferences) > 0 {
		for i, ref := range refs {
			for renamed, original := range transformed.InverseReferences {
				if ref == original {
					refs[i] = renamed
					break
				}
			}
		}
	}
	variant.RawOutput = string(rankingRaw(t, refs))
	return dataset, record, base, variant
}

func TestReplayMetamorphicRestoresBijectionBeforeComparison(t *testing.T) {
	dataset, record, base, variant := metamorphicReportFixture(t, OpaqueReferenceBijection, 601)
	report, err := ReplayMetamorphic(dataset, persistReplayEvidence(t, record, base, variant))
	require.NoError(t, err)
	require.Len(t, report.Observations, 1)
	observation := report.Observations[0]
	require.True(t, *observation.InputChanged)
	require.Equal(t, "passed", observation.Comparison.DeterministicStatus)
	require.True(t, *observation.Comparison.Stable)
	require.Equal(t, "unassessed", observation.Comparison.SemanticStatus)
	require.False(t, report.ReleaseApproved)
}

func TestReplayMetamorphicDoesNotCountNoOpAsEvidence(t *testing.T) {
	dataset, record, base, variant := metamorphicReportFixture(t, CandidatePermutation, 602)
	report, err := ReplayMetamorphic(dataset, persistReplayEvidence(t, record, base, variant))
	require.NoError(t, err)
	require.Len(t, report.Observations, 1)
	observation := report.Observations[0]
	require.False(t, *observation.InputChanged)
	require.Equal(t, "unassessed", observation.Comparison.DeterministicStatus)
	require.Nil(t, observation.Comparison.Stable)
	require.Contains(t, observation.Comparison.Errors, "transformation did not change effective input")
}

func TestReplayMetamorphicDoesNotCountMissingOrInvalidOutputsAsStable(t *testing.T) {
	for _, kind := range []string{"missing", "schema_invalid", "unknown_reference", "duplicate_reference"} {
		t.Run(kind, func(t *testing.T) {
			dataset, record, base, variant := metamorphicReportFixture(t, OpaqueReferenceBijection, 601)
			switch kind {
			case "missing":
				variant = Attempt{CaseID: variant.CaseID, Trial: 1, Status: "not_executed"}
				record.Status = "partial"
			case "schema_invalid":
				variant.RawOutput = `{"recommendations":[{"reference":"unknown"}]}`
			case "unknown_reference":
				variant.RawOutput = string(rankingRaw(t, []string{"unknown"}))
			case "duplicate_reference":
				transformed, err := TransformRanking(dataset.RK[0].Input, OpaqueReferenceBijection, 601)
				require.NoError(t, err)
				ref := transformed.Input.Candidates[0].Reference
				variant.RawOutput = string(rankingRaw(t, []string{ref, ref}))
			}
			report, err := ReplayMetamorphic(dataset, persistReplayEvidence(t, record, base, variant))
			require.NoError(t, err)
			require.Len(t, report.Observations, 1)
			require.Equal(t, "unassessed", report.Observations[0].Comparison.DeterministicStatus)
			require.Nil(t, report.Observations[0].Comparison.Stable)
		})
	}
}

func TestGeminiMetamorphicExecutorSendsTransformedInputWithoutOracle(t *testing.T) {
	dataset, record, _, variant := metamorphicReportFixture(t, OpaqueReferenceBijection, 601)
	dataset.RK[0].Expected = json.RawMessage(`{"private_oracle":"DO_NOT_SEND_EXPECTED_SENTINEL"}`)
	transformed, err := TransformRanking(dataset.RK[0].Input, OpaqueReferenceBijection, 601)
	require.NoError(t, err)
	upstream := &roundTripperMock{}
	providerBody, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": variant.RawOutput}}}}}})
	require.NoError(t, err)
	upstream.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
		request := args.Get(0).(*http.Request)
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NotContains(t, string(body), "DO_NOT_SEND_EXPECTED_SENTINEL")
		require.NotContains(t, string(body), variant.CaseID)
		for _, c := range transformed.Input.Candidates {
			require.Contains(t, string(body), c.Reference)
		}
	}).Return(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(providerBody)))}, nil).Once()
	executor, err := newGeminiExecutor(dataset, record.Plan, executionTestLimits(), true, "fake-key", upstream)
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), variant.CaseID)
	require.NoError(t, err)
	require.Equal(t, 1, output.RequestCount)
	require.Equal(t, variant.RawOutput, output.RawOutput)
	require.NotContains(t, string(output.Input), "DO_NOT_SEND_EXPECTED_SENTINEL")
	require.Equal(t, "a", dataset.RK[0].Input.Candidates[0].Reference)
	upstream.AssertExpectations(t)
}

func TestReplayMetamorphicStableSetDoesNotErasePairwiseFailure(t *testing.T) {
	dataset, record, base, variant := metamorphicReportFixture(t, OpaqueReferenceBijection, 601)
	transformed, err := TransformRanking(dataset.RK[0].Input, OpaqueReferenceBijection, 601)
	require.NoError(t, err)
	variant.RawOutput = string(rankingRaw(t, []string{transformed.Input.Candidates[1].Reference, transformed.Input.Candidates[0].Reference, transformed.Input.Candidates[2].Reference}))
	report, err := ReplayMetamorphic(dataset, persistReplayEvidence(t, record, base, variant))
	require.NoError(t, err)
	require.Len(t, report.Observations, 1)
	comparison := report.Observations[0].Comparison
	require.True(t, *comparison.Stable)
	require.Equal(t, "failed", comparison.DeterministicStatus)
	require.Contains(t, comparison.Errors, "strict_pairwise_violation")
	require.Equal(t, "unassessed", comparison.SemanticStatus)
}
