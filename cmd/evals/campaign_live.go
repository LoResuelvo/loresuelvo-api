package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/google/uuid"
)

type campaignExecutionItem struct {
	Phase     evals.CampaignExecutionPhase
	Model     string
	Plan      *evals.Plan
	Executor  *evals.GeminiExecutor
	Limits    evals.ExecutionLimits
	Directory string
	Reviews   string
}

type campaignLiveRunResult struct {
	Phase          string `json:"phase"`
	RequestedModel string `json:"requested_model"`
	RunID          string `json:"run_id"`
	RunStatus      string `json:"run_status"`
	AttemptsSHA256 string `json:"attempts_sha256"`
	ModelRequests  int    `json:"model_requests"`
}

type campaignLiveResult struct {
	CampaignID            string                       `json:"campaign_id"`
	ProtocolSHA256        string                       `json:"protocol_sha256"`
	DatasetManifestSHA256 string                       `json:"dataset_manifest_sha256"`
	ExecutionSourceCommit string                       `json:"execution_source_commit"`
	PricingVerifiedOn     string                       `json:"pricing_verified_on"`
	Budget                evals.CampaignBudgetEstimate `json:"budget_preflight"`
	BudgetEvidence        evals.CampaignBudgetEvidence `json:"budget_evidence"`
	Runs                  []campaignLiveRunResult      `json:"runs"`
	EvidencePath          string                       `json:"evidence_path"`
	ReleaseApproved       bool                         `json:"release_approved"`
}

func buildCampaignExecutions(dataset *evals.Dataset, config evals.CampaignExecutionConfig, output, apiKey string, allowLive bool) ([]campaignExecutionItem, error) {
	if dataset == nil || output == "" {
		return nil, fmt.Errorf("campaign dataset and output directory are required")
	}
	seenPaths := map[string]bool{}
	items := make([]campaignExecutionItem, 0, len(config.Phases)*len(config.Models))
	requests := 0
	for _, phase := range config.Phases {
		for _, model := range config.Models {
			options := evals.PlanOptions{Suite: phase.Suite, Model: model, Trials: phase.Trials, MaxRetries: config.Execution.MaxRetries, MaxRequests: config.MaximumGenerationCalls, Metamorphic: phase.Metamorphic}
			plan, err := evals.BuildPlan(dataset, options)
			if err != nil {
				return nil, err
			}
			// A run owns only its complete phase/model denominator. The global
			// ceiling is enforced by the shared guard attached after preflight.
			plan.RequestLimit = plan.MaximumRequests
			requests += plan.MaximumRequests
			component := safeCampaignPathComponent(model)
			directory := filepath.Join(output, "runs", phase.Name, component)
			reviews := filepath.Join(output, "reviews", phase.Name, component+".json")
			if seenPaths[directory] {
				return nil, fmt.Errorf("campaign model paths collide")
			}
			seenPaths[directory] = true
			executor, err := evals.NewGeminiExecutor(dataset, plan, config.Execution.Limits, allowLive, apiKey)
			if err != nil {
				return nil, err
			}
			items = append(items, campaignExecutionItem{Phase: phase, Model: model, Plan: plan, Executor: executor, Limits: config.Execution.Limits, Directory: directory, Reviews: reviews})
		}
	}
	if requests != config.MaximumGenerationCalls || requests > evals.CampaignBudgetMaxRequests {
		return nil, fmt.Errorf("campaign plans require %d requests, protocol declares %d", requests, config.MaximumGenerationCalls)
	}
	return items, nil
}

func executeCampaignLive(ctx context.Context, dataset *evals.Dataset, config evals.CampaignExecutionConfig, output, apiKey, pricingVerifiedOn string) (campaignLiveResult, error) {
	result := campaignLiveResult{CampaignID: config.CampaignID, ProtocolSHA256: config.ProtocolSHA256, DatasetManifestSHA256: config.DatasetManifestSHA256, PricingVerifiedOn: pricingVerifiedOn, Runs: []campaignLiveRunResult{}}
	if pricingVerifiedOn != time.Now().UTC().Format(time.DateOnly) {
		return result, fmt.Errorf("pricing must be independently verified on the current UTC execution date")
	}
	if err := verifyCampaignBaseline(config); err != nil {
		return result, err
	}
	commit, err := cleanCommit()
	if err != nil {
		return result, err
	}
	result.ExecutionSourceCommit = commit
	items, err := buildCampaignExecutions(dataset, config, output, apiKey, true)
	if err != nil {
		return result, err
	}
	prices := append([]evals.CampaignPrice(nil), config.Prices...)
	for i := range prices {
		prices[i].Verified = true
	}
	executors := make([]*evals.GeminiExecutor, len(items))
	for i := range items {
		executors[i] = items[i].Executor
	}
	estimate, err := evals.PreflightGeminiCampaignBudget(ctx, executors, prices, config.HardCeilingUSD, config.PricingSource)
	if err != nil {
		return result, err
	}
	result.Budget = estimate
	if err = os.Mkdir(output, 0700); err != nil {
		return result, fmt.Errorf("create new campaign directory: %w", err)
	}

	countTokenRequests := 0
	for _, item := range items {
		countTokenRequests += len(item.Plan.Cases)
	}
	evidence := evals.CampaignEvidence{FormatVersion: "1", CampaignID: config.CampaignID, ProtocolSHA256: config.ProtocolSHA256, DatasetManifestSHA256: config.DatasetManifestSHA256, ExecutionSourceCommit: commit, PricingVerifiedOn: pricingVerifiedOn, BaselineFilesSHA256: config.BaselineFiles, Budget: evals.CampaignBudgetEvidence{
		PricingSource: config.PricingSource, HardCeilingUSD: config.HardCeilingUSD,
		CountTokenRequests: countTokenRequests, ExpectedGenerationCalls: estimate.RequestCount,
		PreflightInputTokens: estimate.InputTokens, ReservedInputTokens: estimate.ReservedInputTokens,
		ReservedOutputTokens: estimate.ReservedOutputTokens, ReservedInputUSD: estimate.ReservedInputUSD,
		ReservedOutputUSD: estimate.ReservedOutputUSD, ReservedTotalUSD: estimate.ReservedTotalUSD, PreflightHeadroomUSD: estimate.HeadroomUSD,
		ProviderUsageComplete: true,
	}}
	for _, phase := range config.Phases {
		evidence.Phases = append(evidence.Phases, evals.CampaignEvidencePhase{Name: phase.Name})
	}
	for _, item := range items {
		currentCommit, cleanErr := cleanCommit()
		if cleanErr != nil {
			return result, fmt.Errorf("campaign source changed after preflight: %w", cleanErr)
		}
		if currentCommit != commit {
			return result, fmt.Errorf("campaign source commit changed after preflight")
		}
		if baselineErr := verifyCampaignBaseline(config); baselineErr != nil {
			return result, baselineErr
		}
		record, report, runErr := executeCampaignItem(ctx, dataset, item, commit, apiKey)
		if runErr != nil {
			return result, runErr
		}
		result.Runs = append(result.Runs, campaignLiveRunResult{Phase: item.Phase.Name, RequestedModel: item.Model, RunID: record.RunID, RunStatus: record.Status, AttemptsSHA256: record.AttemptsSHA256, ModelRequests: report.Requests})
		evidence.Budget.ObservedGenerationCalls += report.Requests
		complete, usageErr := campaignProviderUsageComplete(item.Directory)
		if usageErr != nil {
			return result, usageErr
		}
		evidence.Budget.ProviderUsageComplete = evidence.Budget.ProviderUsageComplete && complete
		for i := range evidence.Phases {
			if evidence.Phases[i].Name != item.Phase.Name {
				continue
			}
			runPath, relErr := filepath.Rel(output, item.Directory)
			if relErr != nil {
				return result, relErr
			}
			reviewPath, relErr := filepath.Rel(output, item.Reviews)
			if relErr != nil {
				return result, relErr
			}
			evidence.Phases[i].Runs = append(evidence.Phases[i].Runs, evals.CampaignRunSpec{RequestedModel: item.Model, RunDirectory: runPath, SemanticReviews: reviewPath})
		}
	}
	used, spentUSD, ok := items[0].Executor.CampaignBudgetSnapshot()
	if !ok {
		return result, fmt.Errorf("campaign budget guard snapshot is unavailable")
	}
	evidence.Budget.GuardRecordedUsageCalls = used
	evidence.Budget.AccountedSpendUSD = spentUSD
	if used != evidence.Budget.ObservedGenerationCalls {
		return result, fmt.Errorf("campaign budget guard usage differs from persisted generation attempts")
	}
	evidencePath := filepath.Join(output, "evidence.json")
	if err = writeExclusiveJSON(evidencePath, evidence); err != nil {
		return result, err
	}
	result.EvidencePath = evidencePath
	result.BudgetEvidence = evidence.Budget
	return result, nil
}

func verifyCampaignBaseline(config evals.CampaignExecutionConfig) error {
	if config.BaselineSourceCommit == "" || len(config.BaselineFiles) == 0 {
		return fmt.Errorf("campaign baseline attestation is missing")
	}
	for path, expected := range config.BaselineFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read baseline file %s: %w", path, err)
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(data))
		if actual != expected {
			return fmt.Errorf("baseline file hash mismatch: %s", path)
		}
	}
	return nil
}

func campaignProviderUsageComplete(directory string) (bool, error) {
	_, attempts, err := evals.ReadRun(directory)
	if err != nil {
		return false, err
	}
	for _, attempt := range attempts {
		if attempt.RequestCount <= 0 {
			continue
		}
		var response struct {
			Usage *struct {
				Input    *int64 `json:"promptTokenCount"`
				Output   *int64 `json:"candidatesTokenCount"`
				Thoughts *int64 `json:"thoughtsTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(attempt.ProviderResponse, &response) != nil || response.Usage == nil || response.Usage.Input == nil || response.Usage.Output == nil || response.Usage.Thoughts == nil {
			return false, nil
		}
	}
	return true, nil
}

func executeCampaignItem(ctx context.Context, dataset *evals.Dataset, item campaignExecutionItem, commit, apiKey string) (evals.RunRecord, evals.Report, error) {
	now := time.Now().UTC()
	record := evals.RunRecord{UnknownDefaults: "Provider defaults not present in captured requested configuration remain unknown; SDK response metadata is recorded only when supplied.", FormatVersion: "1", RunID: uuid.NewString(), Mode: "live", Commit: commit, StartedOn: now, Plan: item.Plan, Limits: item.Limits, Status: "running"}
	journal, err := evals.NewJournal(item.Directory, record)
	if err != nil {
		return record, evals.Report{}, err
	}
	_, runErr := evals.Run(ctx, dataset, item.Plan, record.Limits, item.Executor, journal, record, evals.CredentialRedactor(apiKey))
	if err = errors.Join(runErr, journal.Close()); err != nil {
		return record, evals.Report{}, err
	}
	verified, report, err := evals.Replay(dataset, item.Directory)
	if err != nil {
		return record, report, err
	}
	if err = evals.WriteReport(item.Directory, report); err != nil {
		return verified, report, err
	}
	template, err := evals.NewSemanticReviewTemplate(dataset, item.Directory)
	if err != nil {
		return verified, report, err
	}
	if err = os.MkdirAll(filepath.Dir(item.Reviews), 0700); err != nil {
		return verified, report, err
	}
	if err = evals.WriteSemanticReviews(item.Reviews, template); err != nil {
		return verified, report, err
	}
	return verified, report, nil
}

func safeCampaignPathComponent(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, value)
}

func writeExclusiveJSON(path string, value any) (resultErr error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if _, err = file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}
