package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/google/uuid"
)

// campaignRecoveryAddendum is deliberately decoded separately from the
// executable protocol. The original protocol remains immutable and the
// addendum only describes an append-only overlay.
type campaignRecoveryAddendum struct {
	ProtocolVersion string `json:"protocol_version"`
	AddendumID      string `json:"addendum_id"`
	CampaignID      string `json:"campaign_id"`
	ParentProtocol  string `json:"parent_protocol"`
	Purpose         string `json:"purpose"`
	Scope           struct {
		DatasetManifestSHA256     string `json:"dataset_manifest_sha256"`
		ExecutionSourceCommit     string `json:"execution_source_commit"`
		AllowHoldout              bool   `json:"allow_holdout"`
		QualityOrMalformedRetries bool   `json:"quality_or_malformed_retries"`
	} `json:"scope"`
	Selection struct {
		Unit                            string   `json:"unit"`
		EligibleStatuses                []string `json:"eligible_statuses"`
		RequiresEmptyRawAndParsedOutput bool     `json:"requires_empty_raw_and_parsed_output"`
		EligibleTransientErrors         []string `json:"eligible_transient_errors"`
		ExcludeErrors                   []string `json:"exclude_errors"`
		NeverRetryExecutedResponse      bool     `json:"never_retry_executed_response"`
		NeverMutateOriginalJournal      bool     `json:"never_mutate_original_journal"`
	} `json:"selection"`
	Execution struct {
		MaxAttemptsPerOriginalSlot               int  `json:"max_attempts_per_original_slot"`
		AdditionalAttemptsIncludeOriginalAttempt bool `json:"additional_attempts_include_original_attempt"`
		MaxRecoveryAttemptsTotal                 int  `json:"max_recovery_attempts_total"`
		Concurrency                              int  `json:"concurrency"`
		MaxRetries                               int  `json:"max_retries"`
		AttemptTimeoutSeconds                    int  `json:"attempt_timeout_seconds"`
		MinIntervalSeconds                       int  `json:"min_interval_seconds"`
		Backoff                                  struct {
			Strategy       string `json:"strategy"`
			InitialSeconds int    `json:"initial_seconds"`
			Multiplier     int    `json:"multiplier"`
			MaxSeconds     int    `json:"max_seconds"`
			Jitter         bool   `json:"jitter"`
		} `json:"backoff"`
	} `json:"execution"`
	Budget struct {
		Currency                                              string  `json:"currency"`
		HardCeilingGlobalOriginalPlusRecovery                 float64 `json:"hard_ceiling_global_original_plus_recovery"`
		RequiresOriginalEvidenceBudget                        bool    `json:"requires_original_evidence_budget"`
		RequiresRecoveryPreflightBeforeProviderCall           bool    `json:"requires_recovery_preflight_before_provider_call"`
		ReserveMaxOutputTokens                                int64   `json:"reserve_max_output_tokens"`
		ReserveMaxInputTokensPerRequest                       int64   `json:"reserve_max_input_tokens_per_request"`
		StopBeforeNextRequestIfGlobalUpperBoundExceedsCeiling bool    `json:"stop_before_next_request_if_global_upper_bound_exceeds_ceiling"`
		UnknownUsagePolicy                                    string  `json:"unknown_usage_policy"`
		AdditionalAttemptsAreReportedSeparately               bool    `json:"additional_attempts_are_reported_separately"`
	} `json:"budget"`
	Evidence struct {
		OutputDirectoryMustBeNewPrivatePath            bool `json:"output_directory_must_be_new_private_path"`
		OriginalRunsAreReadOnly                        bool `json:"original_runs_are_read_only"`
		RecordParentRunIDAndAttemptKey                 bool `json:"record_parent_run_id_and_attempt_key"`
		RecordAttemptNumberAndReason                   bool `json:"record_attempt_number_and_reason"`
		RecordSelectionExclusionReason                 bool `json:"record_selection_exclusion_reason"`
		PreserveFailedNotExecutedAndMalformedOriginals bool `json:"preserve_failed_not_executed_and_malformed_originals"`
		SemanticReviewAfterRecovery                    bool `json:"semantic_review_after_recovery"`
		ReleaseApproved                                bool `json:"release_approved"`
	} `json:"evidence"`
	hash string
}

type recoveryCandidate struct {
	Phase                 string `json:"phase"`
	RequestedModel        string `json:"requested_model"`
	RunDirectory          string `json:"run_directory"`
	RunID                 string `json:"run_id"`
	CaseID                string `json:"case_id"`
	Trial                 int    `json:"trial"`
	Retry                 int    `json:"original_retry"`
	Reason                string `json:"reason"`
	OriginalAttemptSHA256 string `json:"original_attempt_sha256"`
}
type campaignRecoveryResult struct {
	CampaignID                    string              `json:"campaign_id"`
	AddendumID                    string              `json:"addendum_id"`
	ProtocolSHA256                string              `json:"protocol_sha256"`
	DatasetManifestSHA256         string              `json:"dataset_manifest_sha256"`
	ExecutionSourceCommit         string              `json:"execution_source_commit"`
	OriginalExecutionSourceCommit string              `json:"original_execution_source_commit"`
	Candidates                    []recoveryCandidate `json:"candidates"`
	AdditionalAttempts            int                 `json:"additional_attempts"`
	MaximumAdditionalAttempts     int                 `json:"maximum_additional_attempts"`
	RecoveryUpperBoundUSD         float64             `json:"recovery_upper_bound_usd"`
	ReleaseApproved               bool                `json:"release_approved"`
	EvidencePath                  string              `json:"evidence_path,omitempty"`
	RecoveryEvidencePaths         []string            `json:"recovery_evidence_paths,omitempty"`
	EvidenceWithRecovery          string              `json:"evidence_with_recovery,omitempty"`
}

func readCampaignRecoveryAddendum(path string) (campaignRecoveryAddendum, error) {
	var a campaignRecoveryAddendum
	if strings.TrimSpace(path) == "" {
		return a, errors.New("--addendum is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return a, err
	}
	if len(data) > 1024*1024 {
		return a, errors.New("recovery addendum is too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return a, fmt.Errorf("decode recovery addendum: %w", err)
	}
	a.hash = fmt.Sprintf("%x", sha256.Sum256(data))
	if a.ProtocolVersion != "1.0.0" || strings.TrimSpace(a.AddendumID) == "" || strings.TrimSpace(a.CampaignID) == "" || a.ParentProtocol == "" {
		return a, errors.New("invalid recovery addendum identity")
	}
	if a.Scope.DatasetManifestSHA256 == "" || a.Scope.ExecutionSourceCommit == "" || a.Scope.AllowHoldout || a.Scope.QualityOrMalformedRetries {
		return a, errors.New("recovery addendum scope is unsafe")
	}
	if !a.Selection.RequiresEmptyRawAndParsedOutput || !a.Selection.NeverRetryExecutedResponse || !a.Selection.NeverMutateOriginalJournal || len(a.Selection.EligibleStatuses) != 1 || a.Selection.EligibleStatuses[0] != "execution_error" || len(a.Selection.EligibleTransientErrors) != 3 || a.Selection.EligibleTransientErrors[0] != "http_503" || a.Selection.EligibleTransientErrors[1] != "timeout" || a.Selection.EligibleTransientErrors[2] != "deadline_exceeded" {
		return a, errors.New("recovery addendum selection policy is unsafe")
	}
	if a.Execution.MaxAttemptsPerOriginalSlot < 2 || a.Execution.MaxAttemptsPerOriginalSlot > evals.RecoveryDefaultMaxAttempts || a.Execution.MaxRecoveryAttemptsTotal <= 0 || a.Execution.Concurrency != 1 || a.Execution.MaxRetries != 0 {
		return a, errors.New("invalid recovery execution bounds")
	}
	if a.Execution.MinIntervalSeconds < 15 || a.Execution.Backoff.Strategy != "exponential" || a.Execution.Backoff.InitialSeconds != 30 || a.Execution.Backoff.Multiplier != 2 || a.Execution.Backoff.MaxSeconds != 300 || a.Execution.Backoff.Jitter {
		return a, errors.New("invalid recovery pacing or backoff policy")
	}
	if a.Budget.Currency != "USD" || a.Budget.HardCeilingGlobalOriginalPlusRecovery <= 0 || a.Budget.ReserveMaxInputTokensPerRequest <= 0 || a.Budget.ReserveMaxOutputTokens <= 0 || !a.Budget.RequiresRecoveryPreflightBeforeProviderCall {
		return a, errors.New("invalid recovery budget policy")
	}
	return a, nil
}

func recoveryTransientReason(a evals.Attempt) string {
	if !evals.RecoverableNoResponse(a) {
		return ""
	}
	return evals.RecoveryReason(a)
}

func collectRecoveryCandidates(dataset *evals.Dataset, spec evals.CampaignReportSpec, addendum campaignRecoveryAddendum) ([]recoveryCandidate, error) {
	if dataset == nil {
		return nil, errors.New("dataset is required")
	}
	if spec.CampaignID != addendum.CampaignID || spec.DatasetManifestSHA256 != addendum.Scope.DatasetManifestSHA256 {
		return nil, errors.New("recovery addendum does not bind to campaign evidence")
	}
	var result []recoveryCandidate
	for _, phase := range spec.Phases {
		for _, run := range phase.Runs {
			record, attempts, err := evals.ReadRun(run.RunDirectory)
			if err != nil {
				return nil, fmt.Errorf("read original run %s: %w", run.RunDirectory, err)
			}
			if record.Commit != addendum.Scope.ExecutionSourceCommit || record.Plan == nil || record.Plan.Model != run.RequestedModel || record.Plan.UsesHoldout {
				return nil, fmt.Errorf("original run %s provenance mismatch", run.RunDirectory)
			}
			seen := map[string]evals.Attempt{}
			for _, attempt := range attempts {
				key := fmt.Sprintf("%s/%d", attempt.CaseID, attempt.Trial)
				prior, ok := seen[key]
				if !ok || attempt.Retry > prior.Retry {
					seen[key] = attempt
				}
			}
			for _, attempt := range seen {
				if reason := recoveryTransientReason(attempt); reason != "" {
					result = append(result, recoveryCandidate{Phase: phase.Name, RequestedModel: run.RequestedModel, RunDirectory: run.RunDirectory, RunID: record.RunID, CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, Reason: reason, OriginalAttemptSHA256: evals.AttemptSHA256(attempt)})
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		for _, pair := range [][2]string{{left.Phase, right.Phase}, {left.RequestedModel, right.RequestedModel}, {left.RunDirectory, right.RunDirectory}, {left.CaseID, right.CaseID}} {
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return left.Trial < right.Trial
	})
	return result, nil
}

func campaignRecoveryUpperBound(calls int, config evals.CampaignExecutionConfig) float64 {
	if calls <= 0 {
		return 0
	}
	var input, output float64
	for _, p := range config.Prices {
		if p.InputUSDPerMillion > input {
			input = p.InputUSDPerMillion
		}
		if p.OutputUSDPerMillion > output {
			output = p.OutputUSDPerMillion
		}
	}
	return float64(calls) * (float64(evals.CampaignBudgetMaxInputTokens)*input + float64(evals.CampaignBudgetMaxOutputTokens)*output) / 1_000_000
}

func maxCampaignInputPrice(prices []evals.CampaignPrice) float64 {
	var max float64
	for _, price := range prices {
		if price.InputUSDPerMillion > max {
			max = price.InputUSDPerMillion
		}
	}
	return max
}

func maxCampaignOutputPrice(prices []evals.CampaignPrice) float64 {
	var max float64
	for _, price := range prices {
		if price.OutputUSDPerMillion > max {
			max = price.OutputUSDPerMillion
		}
	}
	return max
}

func executeCampaignRecovery(ctx context.Context, dataset *evals.Dataset, config evals.CampaignExecutionConfig, addendum campaignRecoveryAddendum, spec evals.CampaignReportSpec, originalEvidencePath, output, apiKey, runnerCommit string) (campaignRecoveryResult, error) {
	result := campaignRecoveryResult{CampaignID: config.CampaignID, AddendumID: addendum.AddendumID, ProtocolSHA256: config.ProtocolSHA256, DatasetManifestSHA256: dataset.ManifestSHA256, ExecutionSourceCommit: runnerCommit, OriginalExecutionSourceCommit: addendum.Scope.ExecutionSourceCommit, Candidates: []recoveryCandidate{}}
	candidates, err := collectRecoveryCandidates(dataset, spec, addendum)
	if err != nil {
		return result, err
	}
	result.Candidates = candidates
	if len(candidates) == 0 {
		return result, errors.New("no eligible no-response transient attempts found")
	}
	if len(candidates) > addendum.Execution.MaxRecoveryAttemptsTotal {
		candidates = candidates[:addendum.Execution.MaxRecoveryAttemptsTotal]
	}
	maxExtra := addendum.Execution.MaxAttemptsPerOriginalSlot - 1
	maxCalls := len(candidates) * maxExtra
	if maxCalls > addendum.Execution.MaxRecoveryAttemptsTotal {
		maxCalls = addendum.Execution.MaxRecoveryAttemptsTotal
	}
	priorUpper := spec.Budget.AccountedSpendUSD
	if !spec.Budget.ProviderUsageComplete {
		priorUpper = spec.Budget.ReservedTotalUSD
	}
	if priorUpper <= 0 || priorUpper > config.HardCeilingUSD {
		return result, errors.New("original evidence has no verifiable global budget bound")
	}
	result.AdditionalAttempts = maxCalls
	result.MaximumAdditionalAttempts = maxCalls
	result.RecoveryUpperBoundUSD = campaignRecoveryUpperBound(maxCalls, config)
	if result.RecoveryUpperBoundUSD+priorUpper > config.HardCeilingUSD {
		return result, fmt.Errorf("recovery conservative upper bound exceeds global ceiling")
	}
	if ctx == nil {
		return result, errors.New("recovery context is required")
	}
	policy := evals.RecoveryPolicy{MaxAttemptsPerSlot: addendum.Execution.MaxAttemptsPerOriginalSlot, InitialBackoff: time.Duration(addendum.Execution.Backoff.InitialSeconds) * time.Second, MaxBackoff: time.Duration(addendum.Execution.Backoff.MaxSeconds) * time.Second}
	if err := policy.Validate(); err != nil {
		return result, err
	}
	priorRequests := spec.Budget.ObservedGenerationCalls
	if priorRequests <= 0 {
		priorRequests = spec.Budget.ExpectedGenerationCalls
	}
	priorSpent := priorUpper
	manifestByRun := map[string]evals.RecoveryManifest{}
	baseAttemptsByRun := map[string][]evals.Attempt{}
	baseRecordsByRun := map[string]evals.RunRecord{}
	for _, candidate := range candidates {
		if _, exists := manifestByRun[candidate.RunDirectory]; exists {
			continue
		}
		base, baseAttempts, readErr := evals.ReadRun(candidate.RunDirectory)
		if readErr != nil {
			return result, readErr
		}
		promptHashes, configHashes := map[string]string{}, map[string]string{}
		for _, attempt := range baseAttempts {
			key := fmt.Sprintf("%s/%d", attempt.CaseID, attempt.Trial)
			if attempt.PromptSHA256 != "" {
				promptHashes[key] = attempt.PromptSHA256
			}
			if len(attempt.GenerationConfig) > 0 {
				configHashes[key] = fmt.Sprintf("%x", sha256.Sum256(attempt.GenerationConfig))
			}
		}
		budget := evals.RecoveryBudget{HardCeilingUSD: config.HardCeilingUSD, PriorRequests: priorRequests, PriorRequestsKnown: true, PriorSpentUSD: &priorSpent, MaxInputTokensPerCall: evals.CampaignBudgetMaxInputTokens, MaxOutputTokensPerCall: evals.CampaignBudgetMaxOutputTokens, InputUSDPerMillion: maxCampaignInputPrice(config.Prices), OutputUSDPerMillion: maxCampaignOutputPrice(config.Prices)}
		binding := evals.RecoveryBinding{DatasetVersion: dataset.Version, DatasetManifestSHA256: dataset.ManifestSHA256, SourceCommit: base.Commit, RequestedModel: base.Plan.Model, BaselineFilesSHA256: config.BaselineFiles, PromptSHA256BySlot: promptHashes, GenerationConfigSHA256BySlot: configHashes}
		manifest, buildErr := evals.BuildRecoveryManifest(base, baseAttempts, binding, policy, budget, newRecoveryID())
		if buildErr != nil {
			return result, buildErr
		}
		manifest.BaseProtocolSHA256 = config.ProtocolSHA256
		manifest.AddendumSHA256 = addendum.hash
		manifest.RecoverySourceCommit = runnerCommit
		if validateErr := evals.ValidateRecovery(evals.RecoveryBundle{Manifest: manifest}, base, baseAttempts); validateErr != nil {
			return result, validateErr
		}
		manifestByRun[candidate.RunDirectory] = manifest
		baseAttemptsByRun[candidate.RunDirectory] = baseAttempts
		baseRecordsByRun[candidate.RunDirectory] = base
	}
	type recoveryWork struct {
		candidate recoveryCandidate
		executors []*evals.GeminiExecutor
	}
	works := make([]recoveryWork, 0, len(candidates))
	allExecutors := make([]*evals.GeminiExecutor, 0, maxCalls)
	prices := append([]evals.CampaignPrice(nil), config.Prices...)
	for i := range prices {
		prices[i].Verified = true
	}
	for _, candidate := range candidates {
		suite := "development"
		if candidate.Phase == "smoke" {
			suite = "smoke"
		}
		work := recoveryWork{candidate: candidate, executors: make([]*evals.GeminiExecutor, 0, maxExtra)}
		for n := 0; n < maxExtra; n++ {
			plan, planErr := evals.BuildPlan(dataset, evals.PlanOptions{Suite: suite, Model: candidate.RequestedModel, Trials: 1, MaxRetries: 0, MaxRequests: 1, CaseIDs: []string{candidate.CaseID}})
			if planErr != nil {
				return result, planErr
			}
			executor, exErr := evals.NewGeminiExecutor(dataset, plan, config.Execution.Limits, true, apiKey)
			if exErr != nil {
				return result, exErr
			}
			work.executors = append(work.executors, executor)
			allExecutors = append(allExecutors, executor)
		}
		works = append(works, work)
	}
	// CountTokens and the shared guard are installed before any GenerateContent call.
	if _, err := evals.PreflightGeminiCampaignBudget(ctx, allExecutors, prices, config.HardCeilingUSD-priorUpper, config.PricingSource); err != nil {
		return result, err
	}
	if err := checkOutputDirectory(dataset.Root, output); err != nil {
		return result, err
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		return result, errors.New("recovery output directory already exists")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		return result, err
	}
	if err := os.MkdirAll(filepath.Join(output, "attempts"), 0700); err != nil {
		return result, err
	}
	if err := os.MkdirAll(filepath.Join(output, "recovery"), 0700); err != nil {
		return result, err
	}
	byRun := map[string][]evals.RecoveryAttempt{}
	actualAttempts := 0
	var lastGeneration time.Time
	for i, work := range works {
		candidate := work.candidate
		for n, executor := range work.executors {
			interval := config.Execution.Limits.MinInterval
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
						return result, ctx.Err()
					case <-timer.C:
					}
				}
			}
			started := time.Now().UTC()
			lastGeneration = started
			attemptCtx, cancel := context.WithTimeout(ctx, config.Execution.Limits.AttemptTimeout)
			out, callErr := executor.Execute(attemptCtx, candidate.CaseID)
			cancel()
			a := evals.Attempt{RequestID: out.RequestID, CaseID: candidate.CaseID, Trial: candidate.Trial, Retry: candidate.Retry + n + 1, StartedOn: started, LatencyMillis: time.Since(started).Milliseconds(), Status: "executed", Input: out.Input, PromptSHA256: out.PromptSHA256, GenerationConfig: out.GenerationConfig, RawOutput: out.RawOutput, ParsedOutput: out.ParsedOutput, ProviderResponse: out.ProviderResponse, RequestCount: out.RequestCount}
			if len(a.Input) > 0 {
				sum := sha256.Sum256(a.Input)
				a.InputSHA256 = fmt.Sprintf("%x", sum)
			}
			if callErr != nil {
				a.Status = "execution_error"
				a.Error = strings.ReplaceAll(callErr.Error(), apiKey, "[REDACTED]")
			}
			item := evals.RecoveryAttempt{Attempt: a, BaseAttemptSHA256: candidate.OriginalAttemptSHA256, RecoveryNumber: n + 1}
			actualAttempts++
			byRun[candidate.RunDirectory] = append(byRun[candidate.RunDirectory], item)
			path := filepath.Join(output, "attempts", fmt.Sprintf("%03d-%s-%s-%d.json", i+1, safeCampaignPathComponent(candidate.RequestedModel), safeCampaignPathComponent(candidate.CaseID), n+1))
			if err := writeExclusiveJSON(path, item); err != nil {
				return result, err
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
					return result, ctx.Err()
				case <-timer.C:
				}
			}
		}
	}
	result.AdditionalAttempts = actualAttempts
	pathByRun := map[string]string{}
	for runDir, attempts := range byRun {
		record, baseAttempts := baseRecordsByRun[runDir], baseAttemptsByRun[runDir]
		manifest, manifestOK := manifestByRun[runDir]
		if !manifestOK {
			return result, fmt.Errorf("missing recovery manifest for %s", runDir)
		}
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

func writeRecoveryEvidenceOverlay(originalPath, output string, recoveryByRun map[string]string) error {
	if strings.TrimSpace(originalPath) == "" {
		return errors.New("original evidence path is required")
	}
	data, err := os.ReadFile(originalPath)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode original evidence: %w", err)
	}
	phases, ok := document["phases"].([]any)
	if !ok {
		return errors.New("original evidence phases are missing")
	}
	for _, rawPhase := range phases {
		phase, ok := rawPhase.(map[string]any)
		if !ok {
			continue
		}
		runs, ok := phase["runs"].([]any)
		if !ok {
			continue
		}
		for _, rawRun := range runs {
			run, ok := rawRun.(map[string]any)
			if !ok {
				continue
			}
			rel, _ := run["run_directory"].(string)
			if rel == "" {
				continue
			}
			resolved := rel
			if !filepath.IsAbs(resolved) {
				resolved, _ = filepath.Abs(filepath.Join(filepath.Dir(originalPath), rel))
			}
			if recovery, found := recoveryByRun[resolved]; found {
				relRecovery, _ := filepath.Rel(filepath.Dir(output), recovery)
				run["recovery_evidence"] = filepath.ToSlash(relRecovery)
			}
		}
	}
	return writeExclusiveJSON(output, document)
}

func newRecoveryID() string { return "campaign-recovery-" + uuid.NewString() }
