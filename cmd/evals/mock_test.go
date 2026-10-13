package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// cliDataset builds a tiny synthetic fixture, not the experimental corpus.
func cliDataset(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"datasets/prediagnosis.jsonl":  `{"id":"PD-001","task":"prediagnosis","family_id":"synthetic","split":"development","input":{"user_message":"test","available_categories":[],"is_new_conversation":true}}` + "\n",
		"datasets/ranking.jsonl":       `{"id":"RK-001","task":"ranking","family_id":"synthetic_ranking","split":"holdout","input":{"problem_title":"test","problem_description":"test","max_results":3,"candidates":[]}}` + "\n",
		"datasets/summarization.jsonl": `{"id":"SM-001","task":"summarization","family_id":"synthetic_summary","split":"holdout","input":{"previous_summary":"","messages":[]},"expected":{}}` + "\n",
		"configs/suites.json":          `{"smoke":["PD-001"],"development":["PD-001"],"holdout":["RK-001","SM-001"],"critical_all":["PD-001"],"rules":{}}`,
		"configs/metamorphic.json":     `{ "ranking": [], "prediagnosis_pairs": [] }`,
		"configs/experiment.json":      `{"trials":{"smoke":1,"baseline_development":3,"release":3}}`,
		"assets/manifest.json":         `{"assets":[]}`,
	}
	for _, name := range []string{"prediagnosis", "ranking", "summarization", "prediagnosis-output", "ranking-output", "summarization-output"} {
		files["schemas/"+name+".schema.json"] = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`
	}
	hashes := make(map[string]string)
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		sum := sha256.Sum256([]byte(content))
		hashes[name] = hex.EncodeToString(sum[:])
	}
	manifest, err := json.Marshal(map[string]any{"version": "synthetic", "hash_algorithm": "SHA-256", "files_sha256": hashes})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), manifest, 0600))
	return root
}

func cliCampaignProtocol(t *testing.T, datasetPath string) string {
	t.Helper()
	dataset, err := evals.LoadDataset(datasetPath)
	require.NoError(t, err)
	document := map[string]any{
		"protocol_version": "1", "campaign_id": "synthetic", "scope": "development", "expected_trials": 3,
		"dataset_version": dataset.Version, "dataset_manifest_sha256": dataset.ManifestSHA256, "allow_holdout": false,
		"solution": map[string]any{"baseline_source_commit": "synthetic-commit", "baseline_files": map[string]string{"go.mod": strings.Repeat("a", 64)}, "require_clean_tree_before_live": true},
		"models":   []any{map[string]any{"requested_model": "model-a"}, map[string]any{"requested_model": "model-b"}},
		"execution": map[string]any{"concurrency": 1, "max_retries": 0, "attempt_timeout_seconds": 1, "global_timeout_seconds": 60, "min_interval_seconds": 1, "max_output_tokens": 4096,
			"primary": map[string]any{"suite": "development", "cases": 1, "trials_per_case": 3, "calls_total": 6},
			"smoke":   map[string]any{"suite": "smoke", "cases": 1, "trials_per_case": 1, "calls_total": 2}, "maximum_provider_calls_total": 8},
		"budget": map[string]any{"currency": "USD", "hard_ceiling": 10, "guard_required_before_live": true, "max_input_tokens_per_request": 20000, "max_output_tokens_per_request": 4096,
			"pricing": map[string]any{"source": "https://example.test/pricing", "must_verify_on_execution_date": true, "model-a": map[string]float64{"input_usd_per_million_tokens": .3, "output_usd_per_million_tokens": 2.5}, "model-b": map[string]float64{"input_usd_per_million_tokens": .3, "output_usd_per_million_tokens": 2.5}}},
	}
	data, err := json.Marshal(document)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "protocol.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func cliRecoveryAddendum(t *testing.T) []byte {
	t.Helper()
	var a campaignRecoveryAddendum
	a.ProtocolVersion = "1.0.0"
	a.AddendumID = "synthetic-recovery"
	a.CampaignID = "synthetic"
	a.ParentProtocol = "protocol.json"
	a.Scope.DatasetManifestSHA256 = strings.Repeat("a", 64)
	a.Scope.ExecutionSourceCommit = "synthetic-commit"
	a.Selection.RequiresEmptyRawAndParsedOutput = true
	a.Selection.NeverRetryExecutedResponse = true
	a.Selection.NeverMutateOriginalJournal = true
	a.Selection.EligibleStatuses = []string{"execution_error"}
	a.Selection.EligibleTransientErrors = []string{"http_503", "timeout", "deadline_exceeded"}
	a.Execution.MaxAttemptsPerOriginalSlot = 5
	a.Execution.MaxRecoveryAttemptsTotal = 360
	a.Execution.Concurrency = 1
	a.Execution.AttemptTimeoutSeconds = 90
	a.Execution.MinIntervalSeconds = 15
	a.Execution.Backoff.Strategy = "exponential"
	a.Execution.Backoff.InitialSeconds = 30
	a.Execution.Backoff.Multiplier = 2
	a.Execution.Backoff.MaxSeconds = 300
	a.Budget.Currency = "USD"
	a.Budget.HardCeilingGlobalOriginalPlusRecovery = 10
	a.Budget.ReserveMaxInputTokensPerRequest = 20000
	a.Budget.ReserveMaxOutputTokens = 4096
	a.Budget.RequiresRecoveryPreflightBeforeProviderCall = true
	data, err := json.Marshal(a)
	require.NoError(t, err)
	return data
}

type recoveryExecutorMock struct{ mock.Mock }

func (m *recoveryExecutorMock) Execute(ctx context.Context, id string) (evals.ExecutionOutput, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(evals.ExecutionOutput), args.Error(1)
}

func cliRecoveryEvidenceFixture(t *testing.T) (campaignRecoveryPreparation, []recoveryCandidate, evals.ExecutionOutput, string, string) {
	t.Helper()
	output := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(output, "attempts"), 0700))
	require.NoError(t, os.Mkdir(filepath.Join(output, "recovery"), 0700))
	runDir := filepath.Join(t.TempDir(), "original-run")
	now := time.Now().UTC()
	plan := &evals.Plan{DatasetVersion: "synthetic", DatasetSHA256: strings.Repeat("a", 64), Model: "model", Trials: 1, RequestLimit: 3, MaximumRequests: 3}
	base := evals.RunRecord{RunID: "base", Commit: "synthetic-commit", Plan: plan, FinishedOn: &now, AttemptsSHA256: strings.Repeat("b", 64)}
	template, err := evals.RedactAttempt(evals.Attempt{Input: json.RawMessage(`[{"role":"user","parts":[{"text":"test"}]}]`), GenerationConfig: json.RawMessage(`{"maxOutputTokens":4096}`), RequestCount: 1}, nil)
	require.NoError(t, err)
	binding := evals.RecoveryBinding{DatasetVersion: plan.DatasetVersion, DatasetManifestSHA256: plan.DatasetSHA256, SourceCommit: base.Commit, RequestedModel: plan.Model, BaselineFilesSHA256: map[string]string{"go.mod": strings.Repeat("c", 64)}, PromptSHA256BySlot: map[string]string{}, GenerationConfigSHA256BySlot: map[string]string{}}
	var originals []evals.Attempt
	var candidates []recoveryCandidate
	for _, id := range []string{"first", "second", "third"} {
		a := template
		a.CaseID = id
		a.Trial = 1
		a.Status = "execution_error"
		a.Error = "503 UNAVAILABLE"
		originals = append(originals, a)
		plan.Cases = append(plan.Cases, evals.PlannedCase{CaseID: id})
		binding.PromptSHA256BySlot[id+"/1"] = template.PromptSHA256
		binding.GenerationConfigSHA256BySlot[id+"/1"] = fmt.Sprintf("%x", sha256.Sum256(template.GenerationConfig))
		candidates = append(candidates, recoveryCandidate{CaseID: id, Trial: 1, RequestedModel: plan.Model, RunDirectory: runDir, OriginalAttemptSHA256: evals.AttemptSHA256(a)})
	}
	prior := .05
	budget := evals.RecoveryBudget{HardCeilingUSD: 10, PriorRequests: 3, PriorRequestsKnown: true, PriorSpentUSD: &prior, MaxInputTokensPerCall: 20000, MaxOutputTokensPerCall: 4096, InputUSDPerMillion: .3, OutputUSDPerMillion: 2.5}
	manifest, err := evals.BuildRecoveryManifest(base, originals, binding, evals.RecoveryPolicy{MaxAttemptsPerSlot: 5, InitialBackoff: 30 * time.Second, MaxBackoff: 300 * time.Second}, budget, "recovery")
	require.NoError(t, err)
	prep := campaignRecoveryPreparation{ManifestByRun: map[string]evals.RecoveryManifest{runDir: manifest}, BaseRecordsByRun: map[string]evals.RunRecord{runDir: base}, BaseAttemptsByRun: map[string][]evals.Attempt{runDir: originals}}
	evidence, err := json.Marshal(map[string]any{"phases": []any{map[string]any{"runs": []any{map[string]any{"run_directory": runDir}}}}})
	require.NoError(t, err)
	originalPath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(originalPath, evidence, 0600))
	out := evals.ExecutionOutput{Input: template.Input, PromptSHA256: template.PromptSHA256, GenerationConfig: template.GenerationConfig, RawOutput: `{"recommendations":[]}`, ParsedOutput: json.RawMessage(`{"recommendations":[]}`), RequestCount: 1}
	return prep, candidates, out, originalPath, output
}
