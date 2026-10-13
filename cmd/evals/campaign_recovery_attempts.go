package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
)

type recoveryWork struct {
	candidate recoveryCandidate
	executors []evals.CaseExecutor
}

// executeRecoveryAttempts preserves the durable physical prefix on dispatch
// stops. A refusal before any RPC is a stop reason, never a physical attempt.
func executeRecoveryAttempts(ctx context.Context, works []recoveryWork, limits evals.ExecutionLimits, addendum campaignRecoveryAddendum, output string, keys []string) (map[string][]evals.RecoveryAttempt, int, string, error) {
	byRun := map[string][]evals.RecoveryAttempt{}
	actualAttempts := 0
	var lastGeneration time.Time
	stopReason := ""
dispatch:
	for i, work := range works {
		candidate := work.candidate
		for n, executor := range work.executors {
			if err := ctx.Err(); err != nil {
				stopReason = recoveryDispatchStopReason(err)
				break dispatch
			}
			interval := limits.MinInterval
			addendumInterval := time.Duration(addendum.Execution.MinIntervalSeconds) * time.Second
			if addendumInterval > interval {
				interval = addendumInterval
			}
			if !lastGeneration.IsZero() {
				if wait := interval - time.Since(lastGeneration); wait > 0 {
					timer := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						timer.Stop()
						stopReason = recoveryDispatchStopReason(ctx.Err())
						break dispatch
					case <-timer.C:
					}
				}
			}
			started := time.Now().UTC()
			lastGeneration = started
			attemptCtx, cancel := context.WithTimeout(ctx, limits.AttemptTimeout)
			out, callErr := executor.Execute(attemptCtx, candidate.CaseID)
			cancel()
			if out.RequestCount == 0 {
				stopReason = recoveryDispatchStopReason(callErr)
				break dispatch
			}
			a := evals.Attempt{Provider: out.Provider, ProviderUsage: out.ProviderUsage, UsageSource: out.UsageSource, HTTPStatus: out.HTTPStatus, RequestID: out.RequestID, CaseID: candidate.CaseID, Trial: candidate.Trial, Retry: candidate.Retry + n + 1, StartedOn: started, LatencyMillis: time.Since(started).Milliseconds(), Status: "executed", Input: out.Input, PromptSHA256: out.PromptSHA256, GenerationConfig: out.GenerationConfig, RawOutput: out.RawOutput, ParsedOutput: out.ParsedOutput, ProviderResponse: out.ProviderResponse, RequestCount: out.RequestCount}
			if len(a.Input) > 0 {
				sum := sha256.Sum256(a.Input)
				a.InputSHA256 = fmt.Sprintf("%x", sum)
			}
			if callErr != nil {
				a.Status = "execution_error"
				a.Error = evals.CredentialsRedactor(keys)(callErr.Error())
			}
			a, redactErr := evals.RedactAttempt(a, keys)
			if redactErr != nil {
				return byRun, actualAttempts, stopReason, redactErr
			}
			item := evals.RecoveryAttempt{Attempt: a, BaseAttemptSHA256: candidate.OriginalAttemptSHA256, RecoveryNumber: n + 1}
			actualAttempts++
			byRun[candidate.RunDirectory] = append(byRun[candidate.RunDirectory], item)
			path := filepath.Join(output, "attempts", fmt.Sprintf("%03d-%s-%s-%d.json", i+1, safeCampaignPathComponent(candidate.RequestedModel), safeCampaignPathComponent(candidate.CaseID), n+1))
			if err := writeExclusiveJSON(path, item); err != nil {
				return byRun, actualAttempts, stopReason, err
			}
			var classified *evals.ExecutionError
			if errors.As(callErr, &classified) && (classified.Stop || classified.LaneStop) {
				stopReason = recoveryDispatchStopReason(callErr)
				break dispatch
			}
			if callErr == nil || strings.TrimSpace(a.RawOutput) != "" || !evals.RecoverableNoResponse(a) {
				break
			}
			if n+1 < len(work.executors) {
				delay := time.Duration(addendum.Execution.Backoff.InitialSeconds) * time.Second
				for j := 1; j < n+1; j++ {
					delay *= time.Duration(addendum.Execution.Backoff.Multiplier)
				}
				maxDelay := time.Duration(addendum.Execution.Backoff.MaxSeconds) * time.Second
				if delay > maxDelay {
					delay = maxDelay
				}
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					stopReason = recoveryDispatchStopReason(ctx.Err())
					break dispatch
				case <-timer.C:
				}
			}
		}
	}
	return byRun, actualAttempts, stopReason, nil
}

func recoveryDispatchStopReason(err error) string {
	var classified *evals.ExecutionError
	if errors.As(err, &classified) && classified.Kind != "" && evals.ValidRecoveryStopReason(classified.Kind) {
		return classified.Kind
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "request_not_dispatched"
}

// finalizeRecoveryEvidence also writes empty partial overlays: a stopped run
// must not lose already durable physical responses or imply full execution.
func finalizeRecoveryEvidence(result campaignRecoveryResult, preparation campaignRecoveryPreparation, byRun map[string][]evals.RecoveryAttempt, originalEvidencePath, output string) (campaignRecoveryResult, error) {
	pathByRun := map[string]string{}
	for runDir := range preparation.ManifestByRun {
		attempts := byRun[runDir]
		record, baseAttempts := preparation.BaseRecordsByRun[runDir], preparation.BaseAttemptsByRun[runDir]
		manifest, manifestOK := preparation.ManifestByRun[runDir]
		if !manifestOK {
			return result, fmt.Errorf("missing recovery manifest for %s", runDir)
		}
		manifest.StopReason = result.StopReason
		bundle := evals.RecoveryBundle{Manifest: manifest, Attempts: attempts}
		if validateErr := evals.ValidateRecovery(bundle, record, baseAttempts); validateErr != nil {
			return result, validateErr
		}
		path := filepath.Join(output, "recovery", safeCampaignPathComponent(record.Plan.Model), safeCampaignPathComponent(record.RunID)+".json")
		if writeErr := evals.WriteCampaignRecovery(path, evals.CampaignRecoveryEvidence{Manifest: manifest, Attempts: attempts}); writeErr != nil {
			return result, writeErr
		}
		result.RecoveryEvidencePaths = append(result.RecoveryEvidencePaths, path)
		pathByRun[runDir] = path
	}
	sort.Strings(result.RecoveryEvidencePaths)
	result.EvidencePath = filepath.Join(output, "recovery.json")
	result.EvidenceWithRecovery = filepath.Join(output, "evidence-with-recovery.json")
	if err := writeExclusiveJSON(result.EvidencePath, result); err != nil {
		return result, err
	}
	if err := writeRecoveryEvidenceOverlay(originalEvidencePath, result.EvidenceWithRecovery, pathByRun); err != nil {
		return result, err
	}
	return result, nil

}
