package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/require"
)

func TestBuildCampaignExecutionsCreatesOneBoundedPlanPerPhaseAndModel(t *testing.T) {
	dataset, err := evals.LoadDataset(cliDataset(t))
	require.NoError(t, err)
	config := evals.CampaignExecutionConfig{
		Models:                 []string{"model-a", "model-b"},
		Phases:                 []evals.CampaignExecutionPhase{{Name: "smoke", Suite: "smoke", Trials: 1}, {Name: "primary", Suite: "development", Trials: 3}},
		Execution:              evals.CampaignExecutionSpec{Limits: evals.ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: time.Minute, MinInterval: time.Second, MaxOutputTokens: evals.CampaignBudgetMaxOutputTokens}},
		MaximumGenerationCalls: 8,
	}
	items, err := buildCampaignExecutions(dataset, config, filepath.Join(t.TempDir(), "campaign"), "not-used", true)
	require.NoError(t, err)
	require.Len(t, items, 4)
	requests := 0
	for _, item := range items {
		requests += item.Plan.MaximumRequests
		require.Equal(t, item.Plan.MaximumRequests, item.Plan.RequestLimit)
		require.Zero(t, item.Plan.MaxRetries)
	}
	require.Equal(t, 8, requests)
}

func TestVerifyCampaignBaselineUsesExactFileHashes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.go")
	data := []byte("package baseline\n")
	require.NoError(t, os.WriteFile(path, data, 0600))
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	config := evals.CampaignExecutionConfig{BaselineSourceCommit: "historical", BaselineFiles: map[string]string{path: hash}}
	require.NoError(t, verifyCampaignBaseline(config))
	config.BaselineFiles[path] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	require.ErrorContains(t, verifyCampaignBaseline(config), "hash mismatch")
}

func TestCampaignLiveDryRunValidatesFrozenProtocolWithoutProviderCalls(t *testing.T) {
	t.Setenv("CHATBOT_API_KEY", "")
	root := filepath.Join("..", "..", "evals")
	var stdout, stderr bytes.Buffer
	code := run([]string{"campaign-live", "--dataset", filepath.Join(root, "datasets", "LoResuelvo_US60_evals_v1.0.0"), "--protocol", filepath.Join(root, "protocols", "campaign-1.json"), "--dry-run"}, &stdout, &stderr)
	require.Zero(t, code, stderr.String())
	require.Contains(t, stdout.String(), `"maximum_generation_calls": 360`)
	require.Contains(t, stdout.String(), `"live_model_calls": 0`)
	require.Contains(t, stdout.String(), `"count_token_calls": 0`)
}

func TestFrozenCampaignBuildsFourPlansWithShared360GenerationCeiling(t *testing.T) {
	root := filepath.Join("..", "..", "evals")
	dataset, err := evals.LoadDataset(filepath.Join(root, "datasets", "LoResuelvo_US60_evals_v1.0.0"))
	require.NoError(t, err)
	config, err := evals.ReadCampaignExecutionConfig(dataset, filepath.Join(root, "protocols", "campaign-1.json"))
	require.NoError(t, err)
	items, err := buildCampaignExecutions(dataset, config, filepath.Join(t.TempDir(), "campaign"), "not-used", true)
	require.NoError(t, err)
	require.Len(t, items, 4)
	requests := 0
	countTokenRequests := 0
	for _, item := range items {
		requests += item.Plan.MaximumRequests
		countTokenRequests += len(item.Plan.Cases)
	}
	require.Equal(t, 360, requests)
	require.Equal(t, 144, countTokenRequests)
}

func TestCampaignLiveRejectsStalePricingBeforeRepositoryOrProviderWork(t *testing.T) {
	_, err := executeCampaignLive(t.Context(), nil, evals.CampaignExecutionConfig{}, filepath.Join(t.TempDir(), "campaign"), "secret", "2000-01-01")
	require.ErrorContains(t, err, "current UTC execution date")
}
