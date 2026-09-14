package evals

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderCoverageSVGIsDeterministicAndShowsUnknowns(t *testing.T) {
	input := []CoverageChartSeries{
		{Name: "model & b", ExpectedSlots: 10, TerminalSlots: 8, ExecutedSlots: 6, FailedSlots: 2, MissingSlots: 2, SemanticUnknown: 3},
		{Name: "model a", ExpectedSlots: 10, TerminalSlots: 10, ExecutedSlots: 9, FailedSlots: 1},
	}
	first, err := RenderCoverageSVG(input)
	require.NoError(t, err)
	second, err := RenderCoverageSVG(input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	body := string(first)
	require.Contains(t, body, "model &amp; b")
	require.Contains(t, body, "terminal 8/10 · executed 6 · failed 2 · missing 2 · semantic unknown 3")
	require.Contains(t, body, "semantic unknown 3")
	require.Less(t, strings.Index(body, "model &amp; b"), strings.Index(body, "model a"))
}

func TestRenderQualitySVGDoesNotTurnUnknownIntoZero(t *testing.T) {
	value := 0.75
	body, err := RenderQualitySVG([]QualityChartSeries{
		{Name: "m", Metric: "accuracy", Summary: MetricSummary{Mean: &value, Observations: 3, ExpectedObservations: 6, EvaluatedBaseCases: 2}},
		{Name: "m", Metric: "ndcg", Summary: MetricSummary{ExpectedObservations: 6}},
	})
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "0.7500")
	require.Contains(t, text, "n=3/6 · base cases=2/6")
	require.Contains(t, text, "ndcg")
	// Unknown metrics must be explicit and cannot be represented by a zero bar.
	require.Contains(t, text, "unknown")
}

func TestRenderOperationsSVGPreservesUnknownMeasurements(t *testing.T) {
	cost := 0.031
	body, err := RenderOperationsSVG([]OperationsChartSeries{{
		Name: "gemini",
		Operations: OperationalSummary{
			Requests:               4,
			LatencyObservations:    2,
			LatencyUnknownRequests: 2,
			Cost:                   &cost,
			InputTokens:            TokenSummary{ObservedTotal: nil, UnknownRequests: 4},
			OutputTokens:           TokenSummary{ObservedTotal: int64Ptr(120), KnownRequests: 4},
		},
	}})
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "latency p50=unknown p95=unknown (n=2/4)")
	require.Contains(t, text, "tokens input=unknown output=120")
	require.Contains(t, text, "cost=0.0310")
}

func TestRenderRankingSVGLabelsBaselinesAndDenominators(t *testing.T) {
	value := 0.5
	body, err := RenderRankingSVG([]RankingChartSeries{
		{Name: "offline rating", Metric: "ndcg_at_3", Summary: MetricSummary{Mean: &value, Observations: 18, ExpectedObservations: 18}, Baseline: true},
		{Name: "model", Metric: "ndcg_at_3", Summary: MetricSummary{ExpectedObservations: 18}},
	})
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "offline rating [baseline]")
	require.Contains(t, text, "n=18/18")
	require.Contains(t, text, "model")
	require.Contains(t, text, "unknown")
}

func TestRenderChartsRejectImpossibleOrNonFiniteData(t *testing.T) {
	_, err := RenderCoverageSVG([]CoverageChartSeries{{Name: "m", ExpectedSlots: 1, TerminalSlots: 2}})
	require.ErrorIs(t, err, ErrInvalidCampaignChartData)
	bad := math.NaN()
	_, err = RenderQualitySVG([]QualityChartSeries{{Name: "m", Summary: MetricSummary{Mean: &bad}}})
	require.ErrorIs(t, err, ErrInvalidCampaignChartData)
	_, err = RenderOperationsSVG([]OperationsChartSeries{{Name: "m", Operations: OperationalSummary{Requests: 1, LatencyObservations: 2}}})
	require.ErrorIs(t, err, ErrInvalidCampaignChartData)
}

func int64Ptr(v int64) *int64 { return &v }

func TestRenderVariabilitySVGShowsPairedDenominators(t *testing.T) {
	mean, maxRange := 0.15, 0.40
	body, err := RenderVariabilitySVG([]TrialVariabilitySeries{{Name: "model", Metric: "ndcg_at_3", ExpectedBaseCases: 18, EvaluatedBaseCases: 17, ComparableBaseCases: 16, VariableBaseCases: 3, MeanWithinCaseRange: &mean, MaxWithinCaseRange: &maxRange}})
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "evaluated=17/18 · comparable=16 · variable=3")
	require.Contains(t, text, "mean within-case range=0.1500 · max within-case range=0.4000")
	require.Contains(t, text, "no population uncertainty")
}

func TestRenderVariabilitySVGRejectsImpossibleRanges(t *testing.T) {
	mean, maxRange := 0.5, 0.2
	_, err := RenderVariabilitySVG([]TrialVariabilitySeries{{Name: "m", ExpectedBaseCases: 1, EvaluatedBaseCases: 2, MeanWithinCaseRange: &mean, MaxWithinCaseRange: &maxRange}})
	require.ErrorIs(t, err, ErrInvalidCampaignChartData)
}
