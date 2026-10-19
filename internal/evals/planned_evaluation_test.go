package evals

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPlannedEvaluationRestoresReferencesWithoutMutatingJournal(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	for _, ref := range []string{"a", "b", "c"} {
		dataset.RK[0].Input.Candidates = append(dataset.RK[0].Input.Candidates, CandidateInput{Reference: ref})
	}
	transformed, err := TransformRanking(dataset.RK[0].Input, OpaqueReferenceBijection, 1)
	require.NoError(t, err)
	refs := []string{}
	for _, c := range transformed.Input.Candidates {
		refs = append(refs, c.Reference)
	}
	raw := string(rankingRaw(t, refs))
	variant := &MetamorphicVariant{BaseCaseID: "RK-test", Transformation: OpaqueReferenceBijection, Seed: 1}
	plan := &Plan{Cases: []PlannedCase{{CaseID: "variant", Variant: variant}}}
	attempt := Attempt{CaseID: "variant", RawOutput: raw}
	result, err := evaluatePlannedAttempt(dataset, plan, attempt)
	require.NoError(t, err)
	require.Equal(t, "passed", result.DeterministicStatus)
	require.Equal(t, "variant", result.CaseID)
	require.Equal(t, raw, attempt.RawOutput)
	require.Equal(t, 1.0, *result.Metrics["ndcg_at_3"].(*float64))
}
func TestPlannedEvaluationRejectsUntransformedReferences(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	for _, ref := range []string{"a", "b", "c"} {
		dataset.RK[0].Input.Candidates = append(dataset.RK[0].Input.Candidates, CandidateInput{Reference: ref})
	}
	plan := &Plan{Cases: []PlannedCase{{CaseID: "variant", Variant: &MetamorphicVariant{BaseCaseID: "RK-test", Transformation: OpaqueReferenceBijection, Seed: 1}}}}
	result, err := evaluatePlannedAttempt(dataset, plan, Attempt{CaseID: "variant", RawOutput: string(rankingRaw(t, []string{"a", "b", "c"}))})
	require.NoError(t, err)
	require.Equal(t, "failed", result.DeterministicStatus)
	require.Contains(t, result.Errors[len(result.Errors)-1], "reference_restoration")
}
func TestReplayRejectsTamperedVariantSeed(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	dataset.RK[0].FamilyID = "family"
	dataset.Metamorphic.Ranking = []RankingTransformationPlan{{BaseCaseID: "RK-test", FamilyID: "family", Split: "development", Transformations: []string{CandidatePermutation}, Seeds: []int64{1}, RepeatTrials: 1, AggregationUnit: "base_case_not_variant"}}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "model-a", Trials: 1, MaxRequests: 2, Metamorphic: true})
	require.NoError(t, err)
	record.Plan = plan
	second := attempt
	second.CaseID = plan.Cases[1].CaseID
	second.RawOutput = string(json.RawMessage(`{"recommendations":[]}`))
	plan.Cases[1].Variant.Seed = 99
	_, _, err = Replay(dataset, persistReplayEvidence(t, record, attempt, second))
	require.ErrorContains(t, err, "recorded plan")
}

func TestSummaryReplayKeepsSemanticCriterionBindings(t *testing.T) {
	dataset := &Dataset{Version: "1", ManifestSHA256: "summary-dataset", SM: []SMCase{{CaseMetadata: CaseMetadata{ID: "SM-one", Task: "summarization", FamilyID: "summary", Split: "development"}, Expected: json.RawMessage(`{"facts_to_preserve":["fact"],"must_not_invent":["cause"],"semantic_assertions":["keep uncertainty"],"required_image_refs":[]}`)}}, Suites: map[string][]string{"smoke": {"SM-one"}}, schemaFiles: evaluationSchemaFixture(t, "summarization-output")}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "offline", Trials: 1, MaxRequests: 1})
	require.NoError(t, err)
	_, record, attempt := replayEvidence(t)
	record.RunID = "summary-replay"
	record.Plan = plan
	attempt.CaseID = "SM-one"
	attempt.RawOutput = `{"summary":"A simulated answer without semantic approval."}`
	attempt.ParsedOutput = json.RawMessage(attempt.RawOutput)
	directory := persistReplayEvidence(t, record, attempt)
	_, report, err := Replay(dataset, directory)
	require.NoError(t, err)
	require.Equal(t, "passed", report.Attempts[0].Evaluation.DeterministicStatus)
	template, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	require.Len(t, template.Reviews, 4)
	require.Equal(t, "summary_assertion_001", template.Reviews[0].CriterionID)
	require.Empty(t, template.Reviews[0].Severity)
	require.NotEmpty(t, template.Reviews[0].CriterionSHA256)
	require.Equal(t, "unassessed", template.Reviews[0].Result)
}
