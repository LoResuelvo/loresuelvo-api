package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/require"
)

func TestReadCampaignRecoveryAddendumRejectsUnknownFieldsAndUnsafeScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "addendum.json")
	canonical, err := os.ReadFile(filepath.Join("..", "..", "evals", "protocols", "campaign-1-recovery-addendum.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, canonical, 0600))
	_, err = readCampaignRecoveryAddendum(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"protocol_version":"1.0.0","addendum_id":"r","campaign_id":"campaign-1","parent_protocol":"campaign-1.json","unexpected":true}`), 0600))
	_, err = readCampaignRecoveryAddendum(path)
	require.Error(t, err)
}

func TestCampaignTwoRecoveryAddendumMatchesReusableLoader(t *testing.T) {
	path := filepath.Join("..", "..", "evals", "protocols", "campaign-2-recovery-addendum.json")
	addendum, err := readCampaignRecoveryAddendum(path)
	require.NoError(t, err)
	require.Equal(t, "campaign-2-recovery-1", addendum.AddendumID)
	require.Equal(t, "campaign-2", addendum.CampaignID)
	require.Equal(t, filepath.Join("evals", "protocols", "campaign-2.json"), addendum.ParentProtocol)
	require.False(t, addendum.Scope.QualityOrMalformedRetries)
	require.True(t, addendum.Selection.NeverRetryExecutedResponse)
}

func TestRecoverySelectionNeverIncludesContentOrNonTransientErrors(t *testing.T) {
	base := evals.Attempt{CaseID: "PD-001", Trial: 1, Retry: 0, Status: "execution_error", Error: "503 UNAVAILABLE", RequestCount: 1}
	require.Equal(t, "transient_provider_unavailable", recoveryTransientReason(base))
	base.RawOutput = `{"malformed":`
	require.Empty(t, recoveryTransientReason(base))
	base.RawOutput = ""
	base.Error = "invalid JSON response"
	require.Empty(t, recoveryTransientReason(base))
	base.Error = "context deadline exceeded"
	require.Equal(t, "transient_request_timeout", recoveryTransientReason(base))
}

func TestCampaignRecoveryUpperBoundUsesHighestConfiguredPrice(t *testing.T) {
	config := evals.CampaignExecutionConfig{Prices: []evals.CampaignPrice{{InputUSDPerMillion: .25, OutputUSDPerMillion: 1.5}, {InputUSDPerMillion: .3, OutputUSDPerMillion: 2.5}}}
	require.InDelta(t, .01624, campaignRecoveryUpperBound(1, config), .000001)
	require.InDelta(t, .06496, campaignRecoveryUpperBound(4, config), .000001)
}

func TestWriteRecoveryEvidenceOverlayResolvesOriginalRelativePaths(t *testing.T) {
	originalDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "recovery-output")
	require.NoError(t, os.MkdirAll(outputDir, 0700))
	originalPath := filepath.Join(originalDir, "evidence.json")
	document := map[string]any{
		"phases": []any{map[string]any{
			"runs": []any{map[string]any{
				"run_directory":             "runs/model-run",
				"semantic_reviews":          "reviews/agent.json",
				"recovery_semantic_reviews": "reviews/recovery-agent.json",
			}},
		}},
	}
	data, err := json.Marshal(document)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(originalPath, data, 0600))
	recoveryPath := filepath.Join(outputDir, "recovery", "model-run.json")
	require.NoError(t, writeRecoveryEvidenceOverlay(originalPath, filepath.Join(outputDir, "evidence-with-recovery.json"), map[string]string{
		filepath.Join(originalDir, "runs/model-run"): recoveryPath,
	}))

	data, err = os.ReadFile(filepath.Join(outputDir, "evidence-with-recovery.json"))
	require.NoError(t, err)
	var written map[string]any
	require.NoError(t, json.Unmarshal(data, &written))
	phases, ok := written["phases"].([]any)
	require.True(t, ok)
	phase, ok := phases[0].(map[string]any)
	require.True(t, ok)
	runs, ok := phase["runs"].([]any)
	require.True(t, ok)
	run, ok := runs[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, filepath.Join(originalDir, "runs/model-run"), run["run_directory"])
	require.Equal(t, filepath.Join(originalDir, "reviews/agent.json"), run["semantic_reviews"])
	require.Equal(t, filepath.Join(originalDir, "reviews/recovery-agent.json"), run["recovery_semantic_reviews"])
	require.Equal(t, "recovery/model-run.json", run["recovery_evidence"])
}
