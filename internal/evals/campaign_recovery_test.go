package evals

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func recoveryHash(char byte) string { return strings.Repeat(string(char), 64) }

func recoveryFixture(t *testing.T) (RunRecord, []Attempt, RecoveryBinding, RecoveryBudget) {
	t.Helper()
	datasetHash := recoveryHash('d')
	plan := &Plan{DatasetVersion: "dataset-v1", DatasetSHA256: datasetHash, Suite: "development", Model: "model-a", Trials: 1, MaxRetries: 0, BaseExecutions: 2, MaximumRequests: 2, RequestLimit: 2, Cases: []PlannedCase{{CaseID: "PD-001", Split: "development"}, {CaseID: "PD-002", Split: "development"}}}
	now := time.Now().UTC()
	record := RunRecord{FormatVersion: resultVersion, RunID: "base-run", Commit: "baseline-runner", StartedOn: now, FinishedOn: &now, Plan: plan, Status: "completed", AttemptsSHA256: recoveryHash('a')}
	failure := Attempt{CaseID: "PD-001", Trial: 1, Status: "execution_error", Error: "execution_error: Error 503, Status: UNAVAILABLE", RequestCount: 1}
	input := json.RawMessage(`[ {"role":"user","parts":[{"text":"resolved"}]} ]`)
	promptHashValue, err := promptHash(input)
	require.NoError(t, err)
	success := Attempt{CaseID: "PD-002", Trial: 1, Status: "executed", Input: input, InputSHA256: digest(input), PromptSHA256: promptHashValue, GenerationConfig: json.RawMessage(`{"maxOutputTokens":4096}`), RawOutput: `{ "ok": true }`, RequestCount: 1}
	binding := RecoveryBinding{DatasetVersion: plan.DatasetVersion, DatasetManifestSHA256: datasetHash, SourceCommit: record.Commit, RequestedModel: plan.Model, BaselineFilesSHA256: map[string]string{"baseline-1.go": recoveryHash('b'), "baseline-2.go": recoveryHash('c'), "baseline-3.go": recoveryHash('e'), "baseline-4.go": recoveryHash('f')}, PromptSHA256BySlot: map[string]string{"PD-001/1": recoveryHash('p')}, GenerationConfigSHA256BySlot: map[string]string{"PD-001/1": digest(json.RawMessage(`{"maxOutputTokens":4096}`))}}
	budget := RecoveryBudget{HardCeilingUSD: 10, PriorRequests: 360, PriorRequestsKnown: true, MaxInputTokensPerCall: CampaignBudgetMaxInputTokens, MaxOutputTokensPerCall: CampaignBudgetMaxOutputTokens, InputUSDPerMillion: .30, OutputUSDPerMillion: 2.5}
	return record, []Attempt{failure, success}, binding, budget
}

func TestBuildRecoveryManifestSelectsOnlyTransientNoResponse(t *testing.T) {
	record, attempts, binding, budget := recoveryFixture(t)
	manifest, err := BuildRecoveryManifest(record, attempts, binding, DefaultRecoveryPolicy(), budget, "recovery-1")
	require.NoError(t, err)
	require.Len(t, manifest.Targets, 1)
	require.Equal(t, "PD-001", manifest.Targets[0].CaseID)
	require.Equal(t, "transient_provider_unavailable", manifest.Targets[0].Reason)
	require.InDelta(t, 5.8464+4*.01624, manifest.Reservation.TotalUpperBoundUSD, 0.000001)
}

func TestRecoverableNoResponseRejectsMalformedAndQualityContent(t *testing.T) {
	base := Attempt{CaseID: "PD-001", Trial: 1, Status: "execution_error", RequestCount: 1, Error: "503 UNAVAILABLE"}
	require.True(t, RecoverableNoResponse(base))
	for _, mutate := range []func(*Attempt){
		func(a *Attempt) { a.RawOutput = `{"malformed":` },
		func(a *Attempt) { a.ProviderResponse = json.RawMessage(`{"candidates":[]}`) },
		func(a *Attempt) { a.ParsedOutput = json.RawMessage(`{"ok":false}`) },
		func(a *Attempt) { a.Status = "asset_error" },
		func(a *Attempt) { a.Error = "validation failed" },
	} {
		candidate := base
		mutate(&candidate)
		require.False(t, RecoverableNoResponse(candidate))
	}
	require.True(t, RecoverableNoResponse(Attempt{Status: "execution_error", RequestCount: 1, Error: "context deadline exceeded"}))
	require.True(t, RecoverableNoResponse(Attempt{Status: "execution_error", RequestCount: 1, Error: "DEADLINE_EXCEEDED"}))
}

func TestRecoveryBudgetChargesUnknownPriorSpendConservatively(t *testing.T) {
	budget := RecoveryBudget{HardCeilingUSD: 10, PriorRequests: 360, PriorRequestsKnown: true, MaxInputTokensPerCall: 20000, MaxOutputTokensPerCall: 4096, InputUSDPerMillion: .30, OutputUSDPerMillion: 2.5}
	reservation, err := budget.Reserve(13)
	require.NoError(t, err)
	require.InDelta(t, 6.05752, reservation.TotalUpperBoundUSD, 0.000001)
	_, err = budget.Reserve(300)
	require.ErrorIs(t, err, ErrRecoveryBudgetExceeded)
}

func TestValidateRecoveryAndEffectiveOverlayPreserveOriginal(t *testing.T) {
	record, attempts, binding, budget := recoveryFixture(t)
	manifest, err := BuildRecoveryManifest(record, attempts, binding, DefaultRecoveryPolicy(), budget, "recovery-1")
	require.NoError(t, err)
	input := json.RawMessage(`[ {"role":"user","parts":[{"text":"resolved"}]} ]`)
	recoveryAttempt := Attempt{CaseID: "PD-001", Trial: 1, Retry: 1, Status: "executed", Input: input, InputSHA256: digest(input), PromptSHA256: binding.PromptSHA256BySlot["PD-001/1"], GenerationConfig: json.RawMessage(`{"maxOutputTokens":4096}`), RawOutput: `{ "ok": true }`, RequestCount: 1}
	bundle := RecoveryBundle{Manifest: manifest, Attempts: []RecoveryAttempt{{Attempt: recoveryAttempt, BaseAttemptSHA256: manifest.Targets[0].OriginalAttemptSHA256, RecoveryNumber: 1}}}
	require.NoError(t, ValidateRecovery(bundle, record, attempts))
	effective, completeness, err := EffectiveRecoveryAttempts(attempts, bundle.Attempts)
	require.NoError(t, err)
	require.True(t, completeness.Complete)
	require.Len(t, effective, 2)
	for _, attempt := range effective {
		if attempt.CaseID == "PD-001" {
			require.Equal(t, 1, attempt.Retry)
			require.Equal(t, "executed", attempt.Status)
		}
	}
	// The source failure remains present and addressable in the base journal.
	require.Equal(t, "execution_error", attempts[0].Status)
}

func TestValidateRecoveryRejectsProvenanceMismatchAndNonEligibleTarget(t *testing.T) {
	record, attempts, binding, budget := recoveryFixture(t)
	manifest, err := BuildRecoveryManifest(record, attempts, binding, DefaultRecoveryPolicy(), budget, "recovery-1")
	require.NoError(t, err)
	manifest.Targets[0].OriginalAttemptSHA256 = recoveryHash('x')
	err = ValidateRecovery(RecoveryBundle{Manifest: manifest}, record, attempts)
	require.ErrorIs(t, err, ErrInvalidCampaignRecovery)
	attempts[0].Error = "malformed JSON response"
	manifest, err = BuildRecoveryManifest(record, attempts, binding, DefaultRecoveryPolicy(), budget, "recovery-2")
	require.NoError(t, err)
	require.Empty(t, manifest.Targets)
}

func TestRecoveryPolicyBackoffIsBounded(t *testing.T) {
	policy := DefaultRecoveryPolicy()
	delay, err := policy.Backoff(4)
	require.NoError(t, err)
	require.Equal(t, 240*time.Second, delay)
	_, err = policy.Backoff(5)
	require.ErrorIs(t, err, ErrInvalidCampaignRecovery)
}

func TestCampaignRecoveryEvidenceIsImmutableAndReadable(t *testing.T) {
	record, attempts, binding, budget := recoveryFixture(t)
	manifest, err := BuildRecoveryManifest(record, attempts, binding, DefaultRecoveryPolicy(), budget, "recovery-1")
	require.NoError(t, err)
	evidence := CampaignRecoveryEvidence{Manifest: manifest}
	path := t.TempDir() + "/recovery.json"
	require.NoError(t, WriteCampaignRecovery(path, evidence))
	loaded, err := ReadCampaignRecovery(path)
	require.NoError(t, err)
	require.Equal(t, manifest.RecoveryID, loaded.Manifest.RecoveryID)
	require.Error(t, WriteCampaignRecovery(path, evidence), "a recovery artifact must not be overwritten")
}

func TestRecoverySemanticReviewsBindOnlyRecoveredOutput(t *testing.T) {
	dataset, record, original := replayEvidence(t)
	dataset.ManifestSHA256 = recoveryHash('d')
	record.Plan.DatasetSHA256 = dataset.ManifestSHA256
	original.Status = "execution_error"
	original.Error = "execution_error: Error 503, Status: UNAVAILABLE"
	original.RawOutput = ""
	original.ParsedOutput = nil
	original.ProviderResponse = nil
	runDirectory := persistReplayEvidence(t, record, original)
	stored, baseAttempts, err := ReadRun(runDirectory)
	require.NoError(t, err)
	generationConfig := json.RawMessage(`{"responseMimeType":"application/json","maxOutputTokens":128}`)
	binding := RecoveryBinding{DatasetVersion: dataset.Version, DatasetManifestSHA256: dataset.ManifestSHA256, SourceCommit: stored.Commit, RequestedModel: stored.Plan.Model, BaselineFilesSHA256: map[string]string{"one": recoveryHash('1'), "two": recoveryHash('2'), "three": recoveryHash('3'), "four": recoveryHash('4')}, PromptSHA256BySlot: map[string]string{"RK-test/1": original.PromptSHA256}, GenerationConfigSHA256BySlot: map[string]string{"RK-test/1": digest(generationConfig)}}
	budget := RecoveryBudget{HardCeilingUSD: 10, PriorRequests: 1, PriorRequestsKnown: true, MaxInputTokensPerCall: 20000, MaxOutputTokensPerCall: 4096, InputUSDPerMillion: .30, OutputUSDPerMillion: 2.50}
	manifest, err := BuildRecoveryManifest(stored, baseAttempts, binding, DefaultRecoveryPolicy(), budget, "recovery-semantic")
	require.NoError(t, err)
	recovery := Attempt{CaseID: "RK-test", Trial: 1, Retry: 1, Status: "executed", Input: original.Input, InputSHA256: digest(original.Input), PromptSHA256: original.PromptSHA256, GenerationConfig: generationConfig, RawOutput: string(rankingRaw(t, []string{"a", "b", "c"})), RequestCount: 1}
	evidence := CampaignRecoveryEvidence{Manifest: manifest, Attempts: []RecoveryAttempt{{Attempt: recovery, BaseAttemptSHA256: manifest.Targets[0].OriginalAttemptSHA256, RecoveryNumber: 1}}}
	template, err := NewRecoverySemanticReviewTemplate(dataset, runDirectory, evidence)
	require.NoError(t, err)
	require.NotEmpty(t, template.Reviews)
	for _, review := range template.Reviews {
		require.Equal(t, 1, review.Retry)
		require.Equal(t, digest([]byte(recovery.RawOutput)), review.OutputSHA256)
	}
	judged := template
	judged.Reviews = append([]SemanticReview(nil), template.Reviews...)
	for i := range judged.Reviews {
		judged.Reviews[i].Result = "pass"
		judged.Reviews[i].EvidenceLocation = "present"
		judged.Reviews[i].EvidenceQuote = recovery.RawOutput
		judged.Reviews[i].Reason = "The recovered response satisfies the bound criterion."
		judged.Reviews[i].Reviewer = "recovery-agent"
		judged.Reviews[i].ReviewerKind = "agent"
		timestamp := time.Date(2026, 9, 14, 15, 0, 0, 0, time.UTC)
		judged.Reviews[i].ReviewedOn = &timestamp
	}
	report, err := ApplyRecoverySemanticReviews(dataset, runDirectory, evidence, judged)
	require.NoError(t, err)
	require.Equal(t, len(template.Reviews), report.ResultCounts["pass"])
	require.Equal(t, len(template.Reviews), report.PendingHumanChecks)
	require.False(t, report.ReleaseApproved)
	judged.Reviews[0].OutputSHA256 = recoveryHash('x')
	_, err = ApplyRecoverySemanticReviews(dataset, runDirectory, evidence, judged)
	require.ErrorIs(t, err, ErrInvalidSemanticReview)
	judged = template
	judged.Reviews = append([]SemanticReview(nil), template.Reviews...)
	judged.Reviews[0].CriterionSHA256 = recoveryHash('x')
	_, err = ApplyRecoverySemanticReviews(dataset, runDirectory, evidence, judged)
	require.ErrorIs(t, err, ErrInvalidSemanticReview)
	judged = template
	judged.Reviews = append([]SemanticReview(nil), template.Reviews...)
	judged.Reviews[0].Retry = 0 // original review namespace cannot be reused
	_, err = ApplyRecoverySemanticReviews(dataset, runDirectory, evidence, judged)
	require.ErrorIs(t, err, ErrInvalidSemanticReview)
}

func TestWriteCampaignRecoveryCanonicalizesRawMessageHashes(t *testing.T) {
	record, attempts, binding, budget := recoveryFixture(t)
	manifest, err := BuildRecoveryManifest(record, attempts, binding, DefaultRecoveryPolicy(), budget, "recovery-canonical")
	require.NoError(t, err)
	rawInput := json.RawMessage(" [ { \"role\": \"user\", \"parts\": [ { \"text\": \"resolved\" } ] } ] ")
	config := json.RawMessage(" { \"maxOutputTokens\": 4096 } ")
	recovered := Attempt{CaseID: "PD-001", Trial: 1, Retry: 1, Status: "executed", Input: rawInput, InputSHA256: digest(rawInput), PromptSHA256: recoveryHash('p'), GenerationConfig: config, RawOutput: `{ "ok": true }`, RequestCount: 1}
	evidence := CampaignRecoveryEvidence{Manifest: manifest, Attempts: []RecoveryAttempt{{Attempt: recovered, BaseAttemptSHA256: manifest.Targets[0].OriginalAttemptSHA256, RecoveryNumber: 1}}}
	path := t.TempDir() + "/recovery.json"
	require.NoError(t, WriteCampaignRecovery(path, evidence))
	loaded, err := ReadCampaignRecovery(path)
	require.NoError(t, err)
	stored := loaded.Attempts[0].Attempt
	require.Equal(t, digest(stored.Input), stored.InputSHA256)
	prompt, err := promptHash(stored.Input)
	require.NoError(t, err)
	require.Equal(t, prompt, stored.PromptSHA256)
	require.Equal(t, digest(stored.GenerationConfig), loaded.Manifest.Binding.GenerationConfigSHA256BySlot["PD-001/1"])
	require.NoError(t, ValidateRecovery(RecoveryBundle(loaded), record, attempts))
}
