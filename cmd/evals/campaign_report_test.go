package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/require"
)

func TestWriteCampaignArtifactsCreatesPracticalTranscriptFreeBundle(t *testing.T) {
	mean := 0.75
	report := evals.CampaignReport{
		Campaign: evals.CampaignProvenance{ID: "campaign-1"},
		Phases: []evals.CampaignPhaseReport{{
			Name: "primary",
			Models: []evals.CampaignModelReport{{
				Provenance: evals.CampaignRunProvenance{RequestedModel: "model-a"},
				Coverage:   evals.CampaignCoverage{ExpectedSlots: 3, TerminalSlots: 3, ExecutedSlots: 2, ExecutionFailedSlots: 1, SemanticUnknownSlots: 2},
				Tasks: map[string]evals.CampaignTaskReport{
					"prediagnosis": {Metrics: map[string]evals.MetricSummary{"accepted_outcome_accuracy": {ExpectedObservations: 2, Observations: 1, Mean: &mean}}},
					"ranking":      {Metrics: map[string]evals.MetricSummary{"ndcg_at_3": {ExpectedObservations: 1, Observations: 1, Mean: &mean}}},
				},
			}},
		}},
		Offline: evals.CampaignOfflineReport{RankingPolicies: []evals.CampaignBaselinePolicyReport{{Policy: evals.BaselinePolicy{Name: "rating_average"}, Metrics: map[string]evals.MetricSummary{"ndcg_at_3": {ExpectedObservations: 1, Observations: 1, Mean: &mean}}}}},
	}
	directory := filepath.Join(t.TempDir(), "report")
	require.NoError(t, writeCampaignArtifacts(directory, report))
	for _, name := range []string{"report.json", "coverage.svg", "quality.svg", "operations.svg", "ranking.svg", "variability.svg"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		require.NoError(t, err)
		require.NotEmpty(t, data)
		require.NotContains(t, string(data), "private transcript")
	}
	require.Error(t, writeCampaignArtifacts(directory, report))
}

func TestRenderCampaignArtifactsRejectsImpossibleCoverage(t *testing.T) {
	_, err := renderCampaignArtifacts(evals.CampaignReport{Phases: []evals.CampaignPhaseReport{{Name: "primary", Models: []evals.CampaignModelReport{{Provenance: evals.CampaignRunProvenance{RequestedModel: "model"}, Coverage: evals.CampaignCoverage{ExpectedSlots: 1, TerminalSlots: 2}}}}}})
	require.ErrorIs(t, err, evals.ErrInvalidCampaignChartData)
}
