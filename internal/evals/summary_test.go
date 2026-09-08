package evals

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetricSummaryWeightsBaseCasesNotRepetitions(t *testing.T) {
	result := summarizeMetric(map[string][]float64{"one": {1, 1, 1}, "two": {0}}, 3, 5)
	require.Equal(t, 0.5, *result.Mean)
	require.Equal(t, 2, result.EvaluatedBaseCases)
	require.Equal(t, 1, result.UnassessedBaseCases)
	require.Equal(t, 4, result.Observations)
	require.Equal(t, 0.0, *result.Minimum)
	require.Equal(t, 1.0, *result.Maximum)
}
func TestMetricSummaryDoesNotInventUndefinedScores(t *testing.T) {
	result := summarizeMetric(nil, 2, 2)
	require.Nil(t, result.Mean)
	require.Nil(t, result.Minimum)
	require.Equal(t, 2, result.UnassessedBaseCases)
}
func TestSummaryTokenSubtotalsExposeMissingMetadata(t *testing.T) {
	now := time.Now()
	result := summarizeOperations([]Attempt{{RequestCount: 1, StartedOn: now, LatencyMillis: 10, ProviderResponse: json.RawMessage(`{"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":0,"totalTokenCount":12}}`)}, {RequestCount: 1, StartedOn: now, LatencyMillis: 20}})
	require.Equal(t, int64(12), *result.InputTokens.ObservedTotal)
	require.Equal(t, 1, result.InputTokens.UnknownRequests)
	require.Equal(t, int64(0), *result.OutputTokens.ObservedTotal)
	require.Nil(t, result.ThoughtTokens.ObservedTotal)
	require.Equal(t, 2, result.ThoughtTokens.UnknownRequests)
	require.Equal(t, 10.0, *result.LatencyP50Millis)
	require.Equal(t, 20.0, *result.LatencyP95Millis)
	require.Nil(t, result.Cost)
}
func TestSummaryAllUnknownOperationsRemainNull(t *testing.T) {
	result := summarizeOperations([]Attempt{{RequestCount: 1}})
	require.Nil(t, result.InputTokens.ObservedTotal)
	require.Nil(t, result.LatencyP50Millis)
	require.Equal(t, 1, result.LatencyUnknownRequests)
}
func TestSummarySingletonMacroF1CountsMissingPredictionsAsFailures(t *testing.T) {
	score, trials := singletonMacroF1(map[string][]outcomeObservation{"one": {{Expected: "self_service", Predicted: "self_service"}}, "two": {{Expected: "professional_required", Predicted: ""}}})
	require.Equal(t, 0.5, *score)
	require.Equal(t, 2, trials)
}
func TestSummaryUsesTerminalRetryWithoutHidingTechnicalFailure(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-one", FamilyID: "family"}}}}
	plan := &Plan{Trials: 1, MaxRetries: 1, Cases: []PlannedCase{{CaseID: "RK-one"}}}
	attempts := []Attempt{{CaseID: "RK-one", Trial: 1, Retry: 0, Status: "execution_error"}, {CaseID: "RK-one", Trial: 1, Retry: 1, Status: "executed"}}
	evaluated := []EvaluatedAttempt{{CaseID: "RK-one", Trial: 1, Retry: 0, ExecutionStatus: "execution_error", Evaluation: Evaluation{CaseID: "RK-one"}}, {CaseID: "RK-one", Trial: 1, Retry: 1, ExecutionStatus: "executed", Evaluation: Evaluation{CaseID: "RK-one", Metrics: map[string]any{"ndcg_at_3": 1.0}}}}
	result, err := Summarize(dataset, plan, attempts, evaluated)
	require.NoError(t, err)
	require.Equal(t, 1, result.BaseCases)
	require.Equal(t, 1, result.Trials)
	require.Equal(t, 2, result.Attempts)
	require.Equal(t, 1, result.ExecutionCounts["execution_error"])
	require.Equal(t, 1.0, *result.Metrics["ndcg_at_3"].Mean)
}
func TestSummaryVariantsRemainOneBaseCase(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-one", FamilyID: "family"}}}}
	plan := &Plan{Trials: 1, MaxRetries: 1, Cases: []PlannedCase{{CaseID: "RK-one"}, {CaseID: "variant", Variant: &MetamorphicVariant{BaseCaseID: "RK-one"}}}}
	attempts := []Attempt{{CaseID: "RK-one", Trial: 1, Status: "executed"}, {CaseID: "variant", Trial: 1, Status: "executed"}}
	evaluated := []EvaluatedAttempt{{CaseID: "RK-one", Trial: 1, ExecutionStatus: "executed", Evaluation: Evaluation{CaseID: "RK-one", Metrics: map[string]any{"ndcg_at_3": 1.0}}}, {CaseID: "variant", Trial: 1, ExecutionStatus: "executed", Evaluation: Evaluation{CaseID: "variant", Metrics: map[string]any{"ndcg_at_3": 0.0}}}}
	result, err := Summarize(dataset, plan, attempts, evaluated)
	require.NoError(t, err)
	require.Equal(t, 1, result.BaseCases)
	require.Equal(t, 1, result.VariantTrials)
	require.Equal(t, 1, result.Metrics["ndcg_at_3"].EvaluatedBaseCases)
	require.Equal(t, 1.0, *result.Metrics["ndcg_at_3"].Mean)
	require.Equal(t, 0.0, *result.VariantMetrics["ndcg_at_3"].Mean)
}
func TestSummaryCriticalCoverageCannotBeApprovedBySmoke(t *testing.T) {
	dataset := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-one", FamilyID: "one"}, Expected: json.RawMessage(`{"outcomes":["professional_required"],"actions":["replace"]}`)}, {CaseMetadata: CaseMetadata{ID: "PD-reserve", FamilyID: "two"}, Expected: json.RawMessage(`{}`)}}, Suites: map[string][]string{"critical_all": {"PD-one", "PD-reserve"}}}
	plan := &Plan{Trials: 1, MaxRetries: 1, Cases: []PlannedCase{{CaseID: "PD-one"}}}
	attempts := []Attempt{{CaseID: "PD-one", Trial: 1, Status: "executed"}}
	evaluated := []EvaluatedAttempt{{CaseID: "PD-one", Trial: 1, ExecutionStatus: "executed", Evaluation: Evaluation{CaseID: "PD-one", DeterministicStatus: "passed", SemanticChecks: []SemanticCheck{{ID: "safety", Result: "unassessed"}}}}}
	result, err := Summarize(dataset, plan, attempts, evaluated)
	require.NoError(t, err)
	require.Equal(t, []string{"PD-reserve"}, result.Critical.MissingCaseIDs)
	require.Equal(t, []string{"PD-one"}, result.Critical.UnassessedCaseIDs)
	require.False(t, result.ReleaseApproved)
}
func TestSummaryRejectsUnmatchedEvaluation(t *testing.T) {
	_, err := Summarize(&Dataset{}, &Plan{}, []Attempt{{CaseID: "unknown"}}, []EvaluatedAttempt{{CaseID: "different"}})
	require.ErrorIs(t, err, ErrInvalidSummary)
}

func TestSummaryRejectsIncompletePlanCoverage(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-one", FamilyID: "one"}}, {CaseMetadata: CaseMetadata{ID: "RK-two", FamilyID: "two"}}}}
	attempts := []Attempt{{CaseID: "RK-one", Trial: 1, Status: "executed"}}
	evaluations := []EvaluatedAttempt{{CaseID: "RK-one", Trial: 1, ExecutionStatus: "executed", Evaluation: Evaluation{CaseID: "RK-one"}}}
	for _, plan := range []*Plan{
		{Trials: 0, Cases: []PlannedCase{{CaseID: "RK-one"}}},
		{Trials: 2, Cases: []PlannedCase{{CaseID: "RK-one"}}},
		{Trials: 1, Cases: []PlannedCase{{CaseID: "RK-one"}, {CaseID: "RK-two"}}},
	} {
		_, err := Summarize(dataset, plan, attempts, evaluations)
		require.ErrorIs(t, err, ErrInvalidSummary)
	}
}
func TestSummaryRetainsExplicitUnexecutedTrials(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-one", FamilyID: "one"}}}}
	plan := &Plan{Trials: 2, Cases: []PlannedCase{{CaseID: "RK-one"}}}
	attempts := []Attempt{{CaseID: "RK-one", Trial: 1, Status: "executed"}, {CaseID: "RK-one", Trial: 2, Status: "not_executed"}}
	evaluations := []EvaluatedAttempt{{CaseID: "RK-one", Trial: 1, ExecutionStatus: "executed", Evaluation: Evaluation{CaseID: "RK-one", Metrics: map[string]any{"ndcg_at_3": 1.0}}}, {CaseID: "RK-one", Trial: 2, ExecutionStatus: "not_executed", Evaluation: Evaluation{CaseID: "RK-one"}}}
	result, err := Summarize(dataset, plan, attempts, evaluations)
	require.NoError(t, err)
	require.Equal(t, 2, result.Trials)
	require.Equal(t, 1, result.ExecutionCounts["not_executed"])
	require.Equal(t, 1, result.Metrics["ndcg_at_3"].UnassessedObservations)
}
func TestSummaryRejectsInvalidRetrySequence(t *testing.T) {
	dataset := &Dataset{RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-one", FamilyID: "one"}}}}
	plan := &Plan{Trials: 1, MaxRetries: 1, Cases: []PlannedCase{{CaseID: "RK-one"}}}
	for _, attempts := range [][]Attempt{
		{{CaseID: "RK-one", Trial: 2, Status: "executed"}},
		{{CaseID: "RK-one", Trial: 1, Retry: 1, Status: "executed"}},
		{{CaseID: "RK-one", Trial: 1, Status: "executed"}, {CaseID: "RK-one", Trial: 1, Retry: 1, Status: "executed"}},
	} {
		evaluations := make([]EvaluatedAttempt, 0, len(attempts))
		for _, a := range attempts {
			evaluations = append(evaluations, EvaluatedAttempt{CaseID: a.CaseID, Trial: a.Trial, Retry: a.Retry, ExecutionStatus: a.Status, Evaluation: Evaluation{CaseID: a.CaseID}})
		}
		_, err := Summarize(dataset, plan, attempts, evaluations)
		require.ErrorIs(t, err, ErrInvalidSummary)
	}
}
