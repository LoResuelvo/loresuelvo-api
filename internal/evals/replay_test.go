package evals

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func replayEvidence(t *testing.T) (*Dataset, RunRecord, Attempt) {
	t.Helper()
	dataset := rankingEvaluationFixture(t)
	dataset.Version = "test-v1"
	dataset.ManifestSHA256 = "manifest"
	dataset.RK[0].Split = "development"
	dataset.Suites = map[string][]string{"smoke": {"RK-test"}}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "model-a", Trials: 1, MaxRequests: 2, MaxRetries: 1})
	require.NoError(t, err)
	now := time.Now().UTC()
	record := RunRecord{FormatVersion: resultVersion, RunID: "test-run", Mode: "live", Commit: "commit", StartedOn: now, FinishedOn: &now, Plan: plan, Status: "completed", Limits: ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: time.Minute, MinInterval: time.Second, MaxOutputTokens: 128}}
	input := json.RawMessage(`[{"role":"user","parts":[{"text":"prompt"}]}]`)
	prompt, err := promptHash(input)
	require.NoError(t, err)
	attempt := Attempt{CaseID: "RK-test", Trial: 1, Status: "executed", Input: input, InputSHA256: digest(input), PromptSHA256: prompt, GenerationConfig: json.RawMessage(`{"responseMimeType":"application/json","maxOutputTokens":128}`), RawOutput: string(rankingRaw(t, []string{"a", "b", "c"})), RequestCount: 1}
	return dataset, record, attempt
}

func persistReplayEvidence(t *testing.T, record RunRecord, attempts ...Attempt) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run")
	journal, err := NewJournal(path, record)
	require.NoError(t, err)
	for _, attempt := range attempts {
		require.NoError(t, journal.Append(attempt))
	}
	require.NoError(t, journal.Finish(&record))
	require.NotEmpty(t, record.AttemptsSHA256)
	require.NoError(t, journal.Close())
	return path
}

func TestReplayPreservesSemanticReviewRequirement(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	_, report, err := Replay(dataset, persistReplayEvidence(t, record, attempt))
	require.NoError(t, err)
	require.Equal(t, 1, report.Requests)
	require.Zero(t, report.DeterministicFailures)
	require.Equal(t, "unassessed", report.SemanticStatus)
	require.False(t, report.ReleaseApproved)
}

func TestReplayRejectsInconsistentEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RunRecord, *[]Attempt)
	}{
		{"missing trial", func(_ *RunRecord, a *[]Attempt) { *a = nil }},
		{"duplicate trial", func(_ *RunRecord, a *[]Attempt) { *a = append(*a, (*a)[0]) }},
		{"unknown case", func(_ *RunRecord, a *[]Attempt) { (*a)[0].CaseID = "RK-unknown" }},
		{"input hash", func(_ *RunRecord, a *[]Attempt) { (*a)[0].InputSHA256 = "bad" }},
		{"prompt hash", func(_ *RunRecord, a *[]Attempt) { (*a)[0].PromptSHA256 = "bad" }},
		{"missing first retry", func(_ *RunRecord, a *[]Attempt) { (*a)[0].Retry = 1 }},
		{"invalid request count", func(_ *RunRecord, a *[]Attempt) { (*a)[0].RequestCount = 2 }},
		{"unknown status", func(_ *RunRecord, a *[]Attempt) { (*a)[0].Status = "invented" }},
		{"dataset mismatch", func(r *RunRecord, _ *[]Attempt) { r.Plan.DatasetSHA256 = "other" }},
		{"altered plan", func(r *RunRecord, _ *[]Attempt) { r.Plan.Cases[0].Split = "holdout" }},
		{"missing executed input", func(_ *RunRecord, a *[]Attempt) {
			(*a)[0].Input = nil
			(*a)[0].InputSHA256 = ""
			(*a)[0].PromptSHA256 = ""
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataset, record, attempt := replayEvidence(t)
			attempts := []Attempt{attempt}
			tc.mutate(&record, &attempts)
			_, _, err := Replay(dataset, persistReplayEvidence(t, record, attempts...))
			require.Error(t, err)
		})
	}
}

func TestReplayPreservesPartialRunWithoutRequests(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	record.Status = "partial"
	attempt = Attempt{CaseID: attempt.CaseID, Trial: 1, Status: "not_executed", Error: "context canceled"}
	_, report, err := Replay(dataset, persistReplayEvidence(t, record, attempt))
	require.NoError(t, err)
	require.Zero(t, report.Requests)
	require.Equal(t, 1, report.ExecutionCounts["not_executed"])
	require.False(t, report.ReleaseApproved)
}

func TestCompareAllowsOnlyCompatibleModelChange(t *testing.T) {
	dataset, left, attempt := replayEvidence(t)
	leftPath := persistReplayEvidence(t, left, attempt)
	_, right, rightAttempt := replayEvidence(t)
	right.Plan.Model = "model-b"
	result, err := Compare(dataset, leftPath, persistReplayEvidence(t, right, rightAttempt))
	require.NoError(t, err)
	require.Equal(t, "model-b", result.RightModel)
	require.Len(t, result.Pairs, 1)
	require.False(t, result.ReleaseApproved)
	for _, name := range []string{"commit", "limits", "input", "generation_config"} {
		t.Run(name, func(t *testing.T) {
			_, record, a := replayEvidence(t)
			switch name {
			case "commit":
				record.Commit = "other"
			case "limits":
				record.Limits.MaxOutputTokens++
			case "input":
				a.Input = json.RawMessage(`[{"parts":[{"text":"different prompt"}]}]`)
				a.InputSHA256 = digest(a.Input)
				a.PromptSHA256, err = promptHash(a.Input)
				require.NoError(t, err)
			case "generation_config":
				a.GenerationConfig = json.RawMessage(`{"responseMimeType":"application/json","maxOutputTokens":128,"temperature":1}`)
			}
			_, err := Compare(dataset, leftPath, persistReplayEvidence(t, record, a))
			require.Error(t, err)
		})
	}
}
