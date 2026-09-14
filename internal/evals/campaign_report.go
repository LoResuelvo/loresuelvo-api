package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	campaignReportVersion = "1"
	maxCampaignSpecBytes  = 1024 * 1024
)

var ErrInvalidCampaignReport = errors.New("invalid campaign report")

// CampaignReportSpec declares the complete evidence set. It is intentionally
// independent from a particular campaign size: denominators come from each
// verified run plan, while phase metadata must agree with that plan.
type CampaignReportSpec struct {
	FormatVersion         string                 `json:"format_version"`
	CampaignID            string                 `json:"campaign_id"`
	DatasetVersion        string                 `json:"dataset_version"`
	DatasetManifestSHA256 string                 `json:"dataset_manifest_sha256"`
	SourceCommit          string                 `json:"source_commit"`
	Execution             CampaignExecutionSpec  `json:"execution"`
	PricingVerifiedOn     string                 `json:"pricing_verified_on"`
	BaselineFilesSHA256   map[string]string      `json:"baseline_files_sha256"`
	Budget                CampaignBudgetEvidence `json:"budget"`
	Phases                []CampaignPhaseSpec    `json:"phases"`
	ProtocolSHA256        string                 `json:"-"`
}

type CampaignExecutionSpec struct {
	MaxRetries int             `json:"max_retries"`
	Limits     ExecutionLimits `json:"limits"`
}

type CampaignPhaseSpec struct {
	Name        string            `json:"name"`
	Suite       string            `json:"suite"`
	Trials      int               `json:"trials"`
	Metamorphic bool              `json:"metamorphic"`
	Runs        []CampaignRunSpec `json:"runs"`
}

type CampaignRunSpec struct {
	RequestedModel          string `json:"requested_model"`
	RunDirectory            string `json:"run_directory"`
	SemanticReviews         string `json:"semantic_reviews,omitempty"`
	RecoveryEvidence        string `json:"recovery_evidence,omitempty"`
	RecoverySemanticReviews string `json:"recovery_semantic_reviews,omitempty"`
}

// CampaignEvidence is the local, post-commit binding between a frozen protocol
// and immutable run/review files. It belongs with ignored run evidence, not in
// the versioned protocol.
type CampaignEvidence struct {
	FormatVersion         string                  `json:"format_version"`
	CampaignID            string                  `json:"campaign_id"`
	ProtocolSHA256        string                  `json:"protocol_sha256"`
	DatasetManifestSHA256 string                  `json:"dataset_manifest_sha256"`
	ExecutionSourceCommit string                  `json:"execution_source_commit"`
	PricingVerifiedOn     string                  `json:"pricing_verified_on"`
	BaselineFilesSHA256   map[string]string       `json:"baseline_files_sha256"`
	Budget                CampaignBudgetEvidence  `json:"budget"`
	Phases                []CampaignEvidencePhase `json:"phases"`
}

type CampaignBudgetEvidence struct {
	PricingSource           string  `json:"pricing_source"`
	HardCeilingUSD          float64 `json:"hard_ceiling_usd"`
	CountTokenRequests      int     `json:"count_token_requests"`
	ExpectedGenerationCalls int     `json:"expected_generation_calls"`
	PreflightInputTokens    int64   `json:"preflight_input_tokens"`
	ReservedInputTokens     int64   `json:"reserved_input_tokens"`
	ReservedOutputTokens    int64   `json:"reserved_output_tokens"`
	ReservedInputUSD        float64 `json:"reserved_input_usd"`
	ReservedOutputUSD       float64 `json:"reserved_output_usd"`
	ReservedTotalUSD        float64 `json:"reserved_total_usd"`
	PreflightHeadroomUSD    float64 `json:"preflight_headroom_usd"`
	ObservedGenerationCalls int     `json:"observed_generation_calls"`
	GuardRecordedUsageCalls int     `json:"guard_recorded_usage_calls"`
	AccountedSpendUSD       float64 `json:"accounted_spend_usd"`
	ProviderUsageComplete   bool    `json:"provider_usage_complete"`
}

type CampaignEvidencePhase struct {
	Name string            `json:"name"`
	Runs []CampaignRunSpec `json:"runs"`
}

type campaignProtocol struct {
	ProtocolVersion       string                    `json:"protocol_version"`
	CampaignID            string                    `json:"campaign_id"`
	Status                string                    `json:"status"`
	Purpose               string                    `json:"purpose"`
	DatasetVersion        string                    `json:"dataset_version"`
	DatasetManifestSHA256 string                    `json:"dataset_manifest_sha256"`
	SourceCommit          string                    `json:"source_commit,omitempty"`
	Scope                 string                    `json:"scope"`
	ExpectedTrials        int                       `json:"expected_trials"`
	AllowHoldout          bool                      `json:"allow_holdout"`
	Dataset               json.RawMessage           `json:"dataset"`
	Solution              campaignProtocolSolution  `json:"solution"`
	Models                []campaignProtocolModel   `json:"models"`
	Execution             campaignProtocolExecution `json:"execution"`
	Budget                campaignProtocolBudget    `json:"budget"`
	Measurement           json.RawMessage           `json:"measurement"`
	Traceability          json.RawMessage           `json:"traceability"`
}

type campaignProtocolModel struct {
	RequestedModel string `json:"requested_model"`
}

type campaignProtocolSolution struct {
	BaselineSourceCommit          string            `json:"baseline_source_commit"`
	BaselineFiles                 map[string]string `json:"baseline_files"`
	ParentCampaignID              string            `json:"parent_campaign_id,omitempty"`
	ChangeDescription             string            `json:"change_description,omitempty"`
	RequireCleanTreeBeforeLive    bool              `json:"require_clean_tree_before_live"`
	PromptChangeAllowed           bool              `json:"prompt_change_allowed"`
	GenerationConfigChangeAllowed bool              `json:"generation_config_change_allowed"`
	QualityFixesAllowed           bool              `json:"quality_fixes_allowed"`
}

type campaignProtocolExecution struct {
	Concurrency           int                   `json:"concurrency"`
	MaxRetries            int                   `json:"max_retries"`
	RetryForQuality       bool                  `json:"retry_for_quality"`
	AttemptTimeoutSeconds int                   `json:"attempt_timeout_seconds"`
	GlobalTimeoutSeconds  int                   `json:"global_timeout_seconds"`
	MinIntervalSeconds    int                   `json:"min_interval_seconds"`
	MaxOutputTokens       int                   `json:"max_output_tokens"`
	Primary               campaignProtocolPhase `json:"primary"`
	Smoke                 campaignProtocolPhase `json:"smoke"`
	MaximumProviderCalls  int                   `json:"maximum_provider_calls_total"`
	Contracts             json.RawMessage       `json:"contracts"`
	RankingBaselines      json.RawMessage       `json:"ranking_baselines"`
	Metamorphic           struct {
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason,omitempty"`
	} `json:"metamorphic"`
}

type campaignProtocolPhase struct {
	Suite                     string `json:"suite"`
	Cases                     int    `json:"cases"`
	TrialsPerCase             int    `json:"trials_per_case"`
	CallsTotal                int    `json:"calls_total"`
	IncludedInCampaignCeiling bool   `json:"included_in_campaign_ceiling,omitempty"`
}

type campaignProtocolBudget struct {
	Currency                  string          `json:"currency"`
	HardCeiling               float64         `json:"hard_ceiling"`
	GuardRequiredBeforeLive   bool            `json:"guard_required_before_live"`
	GuardPolicy               string          `json:"guard_policy"`
	MaxInputTokensPerRequest  int64           `json:"max_input_tokens_per_request"`
	MaxOutputTokensPerRequest int64           `json:"max_output_tokens_per_request"`
	PreflightUpperBoundUSD    float64         `json:"preflight_upper_bound_usd"`
	PreflightCalculation      string          `json:"preflight_calculation"`
	PreflightHeadroomUSD      float64         `json:"preflight_headroom_usd"`
	Pricing                   json.RawMessage `json:"pricing"`
	UnknownUsagePolicy        string          `json:"unknown_usage_policy"`
}

type campaignProtocolPrice struct {
	InputUSDPerMillionTokens  float64 `json:"input_usd_per_million_tokens"`
	OutputUSDPerMillionTokens float64 `json:"output_usd_per_million_tokens"`
}

type CampaignExecutionPhase struct {
	Name        string `json:"name"`
	Suite       string `json:"suite"`
	Trials      int    `json:"trials"`
	Metamorphic bool   `json:"metamorphic"`
}

// CampaignExecutionConfig is the executable subset of a frozen protocol.
// Prices remain unverified until the CLI receives an execution-date attestation.
type CampaignExecutionConfig struct {
	CampaignID             string                   `json:"campaign_id"`
	ProtocolSHA256         string                   `json:"protocol_sha256"`
	DatasetVersion         string                   `json:"dataset_version"`
	DatasetManifestSHA256  string                   `json:"dataset_manifest_sha256"`
	Models                 []string                 `json:"models"`
	Phases                 []CampaignExecutionPhase `json:"phases"`
	Execution              CampaignExecutionSpec    `json:"execution"`
	MaximumGenerationCalls int                      `json:"maximum_generation_calls"`
	HardCeilingUSD         float64                  `json:"hard_ceiling_usd"`
	PricingSource          string                   `json:"pricing_source"`
	Prices                 []CampaignPrice          `json:"prices"`
	BaselineSourceCommit   string                   `json:"baseline_source_commit"`
	BaselineFiles          map[string]string        `json:"baseline_files_sha256"`
	ParentCampaignID       string                   `json:"parent_campaign_id,omitempty"`
	ChangeDescription      string                   `json:"change_description,omitempty"`
}

type CampaignProvenance struct {
	ID                    string `json:"id"`
	DatasetVersion        string `json:"dataset_version"`
	DatasetManifestSHA256 string `json:"dataset_manifest_sha256"`
	SourceCommit          string `json:"source_commit"`
	ProtocolSHA256        string `json:"protocol_sha256"`
	PricingVerifiedOn     string `json:"pricing_verified_on"`
}

type CampaignCoverage struct {
	ExpectedSlots        int  `json:"expected_slots"`
	TerminalSlots        int  `json:"terminal_slots"`
	ExecutedSlots        int  `json:"executed_slots"`
	ExecutionFailedSlots int  `json:"execution_failed_slots"`
	DeterministicPassed  int  `json:"deterministic_passed_slots"`
	DeterministicFailed  int  `json:"deterministic_failed_slots"`
	SemanticUnknownSlots int  `json:"semantic_unknown_slots"`
	SemanticFailedSlots  int  `json:"semantic_failed_slots"`
	AgentReviewedSlots   int  `json:"agent_reviewed_slots"`
	HumanReviewedSlots   int  `json:"human_reviewed_slots"`
	RecoveredSlots       int  `json:"recovered_slots"`
	EvidenceComplete     bool `json:"evidence_complete"`
	ResponsesComplete    bool `json:"responses_complete"`
	SemanticsComplete    bool `json:"semantics_complete"`
	HumanReviewComplete  bool `json:"human_review_complete"`
}

type CampaignStatus struct {
	CampaignCoverage
	Phases int `json:"phases"`
	Models int `json:"models"`
}

type CampaignRunProvenance struct {
	RunID                        string          `json:"run_id"`
	RunStatus                    string          `json:"run_status"`
	StartedOn                    time.Time       `json:"started_on"`
	FinishedOn                   *time.Time      `json:"finished_on"`
	AttemptsSHA256               string          `json:"attempts_sha256"`
	RequestedModel               string          `json:"requested_model"`
	RequestLimit                 int             `json:"request_limit"`
	MaximumRequests              int             `json:"maximum_requests"`
	Limits                       ExecutionLimits `json:"limits"`
	ResolvedModelVersions        []string        `json:"resolved_model_versions"`
	UnknownResolvedModelRequests int             `json:"unknown_resolved_model_requests"`
	PromptSHA256                 []string        `json:"prompt_sha256"`
	InputSHA256                  []string        `json:"input_sha256"`
	GenerationConfigSHA256       []string        `json:"generation_config_sha256"`
}

type CampaignSemanticSummary struct {
	ReviewSource         string         `json:"review_source"`
	ReviewDocumentSHA256 string         `json:"review_document_sha256,omitempty"`
	ReviewerKinds        []string       `json:"reviewer_kinds"`
	CriterionCounts      map[string]int `json:"criterion_counts"`
	PendingHumanChecks   int            `json:"pending_human_checks"`
}

type CampaignTaskReport struct {
	Coverage         CampaignCoverage             `json:"coverage"`
	Metrics          map[string]MetricSummary     `json:"metrics"`
	TrialVariability map[string]MetricVariability `json:"trial_variability"`
	SingletonMacroF1 *float64                     `json:"singleton_macro_f1,omitempty"`
	SingletonCases   int                          `json:"singleton_base_cases,omitempty"`
	SingletonTrials  int                          `json:"singleton_trials,omitempty"`
}

// MetricVariability describes within-case trial ranges. It is descriptive:
// repeated trials are not treated as independent population samples.
type MetricVariability struct {
	ExpectedBaseCases   int      `json:"expected_base_cases"`
	EvaluatedBaseCases  int      `json:"evaluated_base_cases"`
	ComparableBaseCases int      `json:"comparable_base_cases"`
	VariableBaseCases   int      `json:"variable_base_cases"`
	MeanWithinCaseRange *float64 `json:"mean_within_case_range"`
	MaxWithinCaseRange  *float64 `json:"max_within_case_range"`
}

type CampaignBaselinePolicyReport struct {
	Policy                 BaselinePolicy           `json:"policy"`
	Metrics                map[string]MetricSummary `json:"metrics"`
	DeterministicFailedIDs []string                 `json:"deterministic_failed_case_ids"`
	SemanticUnknownCases   int                      `json:"semantic_unknown_cases"`
}

type CampaignOfflineReport struct {
	Contracts            []ContractResult               `json:"contracts"`
	ContractCounts       map[string]int                 `json:"contract_counts"`
	RankingBaseCases     int                            `json:"ranking_base_cases"`
	RankingModelRequests int                            `json:"ranking_model_requests"`
	RankingPolicies      []CampaignBaselinePolicyReport `json:"ranking_policies"`
}

type CampaignModelReport struct {
	Provenance CampaignRunProvenance         `json:"provenance"`
	Coverage   CampaignCoverage              `json:"coverage"`
	Tasks      map[string]CampaignTaskReport `json:"tasks"`
	Operations OperationalSummary            `json:"operations"`
	Semantic   CampaignSemanticSummary       `json:"semantic"`
	Recovery   *CampaignRecoverySummary      `json:"recovery,omitempty"`
}

// CampaignRecoverySummary reports the append-only overlay without replacing
// the immutable base journal or folding recovery calls into base operations.
type CampaignRecoverySummary struct {
	RecoveryID              string         `json:"recovery_id"`
	EvidenceSHA256          string         `json:"evidence_sha256"`
	TargetSlots             int            `json:"target_slots"`
	RecoveryAttempts        int            `json:"recovery_attempts"`
	RecoveredSlots          int            `json:"recovered_slots"`
	MissingSlots            []string       `json:"missing_slots"`
	AttemptStatuses         map[string]int `json:"attempt_statuses"`
	ReservedAdditionalCalls int            `json:"reserved_additional_calls"`
	AdditionalUpperBoundUSD float64        `json:"additional_upper_bound_usd"`
	AccountedAdditionalUSD  float64        `json:"accounted_additional_spend_usd"`
	ProviderUsageComplete   bool           `json:"provider_usage_complete"`
	SemanticReviewSHA256    string         `json:"semantic_review_sha256,omitempty"`
	SemanticReviewerKinds   []string       `json:"semantic_reviewer_kinds"`
	SemanticPendingHuman    int            `json:"semantic_pending_human_checks"`
}

type CampaignRecoveryReport struct {
	EvidenceDocuments        int     `json:"evidence_documents"`
	TargetSlots              int     `json:"target_slots"`
	RecoveryAttempts         int     `json:"recovery_attempts"`
	RecoveredSlots           int     `json:"recovered_slots"`
	MissingSlots             int     `json:"missing_slots"`
	ReservedAdditionalCalls  int     `json:"reserved_additional_calls"`
	AdditionalUpperBoundUSD  float64 `json:"additional_upper_bound_usd"`
	OriginalReservedUpperUSD float64 `json:"original_reserved_upper_bound_usd"`
	CombinedUpperBoundUSD    float64 `json:"combined_upper_bound_usd"`
	OriginalAccountedUSD     float64 `json:"original_accounted_spend_usd"`
	AdditionalAccountedUSD   float64 `json:"additional_accounted_spend_usd"`
	CombinedAccountedUSD     float64 `json:"combined_accounted_spend_usd"`
	ProviderUsageComplete    bool    `json:"provider_usage_complete"`
	HardCeilingUSD           float64 `json:"hard_ceiling_usd"`
	WithinHardCeiling        bool    `json:"within_hard_ceiling"`
}

type campaignRecoveryOverlay struct {
	slots                map[string]RecoveredAttempt
	evidence             CampaignRecoveryEvidence
	evidenceSHA256       string
	reviewsByCriterion   map[string]SemanticReview
	reviewerKindsBySlot  map[string]map[string]bool
	semanticReviewSHA256 string
	semanticPendingHuman int
}

type CampaignPhaseReport struct {
	Name        string                `json:"name"`
	Suite       string                `json:"suite"`
	Trials      int                   `json:"trials"`
	Metamorphic bool                  `json:"metamorphic"`
	Coverage    CampaignCoverage      `json:"coverage"`
	Models      []CampaignModelReport `json:"models"`
}

// CampaignCaseTrial is deliberately transcript-free. It supports failure-mode
// analysis while binding every row to the exact output and reviewed criteria.
type CampaignCaseTrial struct {
	Phase                   string         `json:"phase"`
	RequestedModel          string         `json:"requested_model"`
	CaseID                  string         `json:"case_id"`
	BaseCaseID              string         `json:"base_case_id"`
	Task                    string         `json:"task"`
	FamilyID                string         `json:"family_id"`
	Split                   string         `json:"split"`
	Trial                   int            `json:"trial"`
	TerminalRetry           int            `json:"terminal_retry"`
	Attempts                int            `json:"attempts_including_retries"`
	ExecutionStatus         string         `json:"execution_status"`
	OriginalExecutionStatus string         `json:"original_execution_status"`
	OriginalFailureCodes    []string       `json:"original_failure_codes"`
	Recovered               bool           `json:"recovered"`
	RecoveryReason          string         `json:"recovery_reason,omitempty"`
	RecoveryAttempts        int            `json:"recovery_attempts"`
	OriginalAttemptSHA256   string         `json:"original_attempt_sha256,omitempty"`
	RecoveryEvidenceSHA256  string         `json:"recovery_evidence_sha256,omitempty"`
	RecoveryAttemptStatuses []string       `json:"recovery_attempt_statuses,omitempty"`
	DeterministicStatus     string         `json:"deterministic_status"`
	OverallStatus           string         `json:"overall_status"`
	SemanticStatus          string         `json:"semantic_status"`
	FailureCodes            []string       `json:"failure_codes"`
	Metrics                 map[string]any `json:"metrics"`
	SemanticCounts          map[string]int `json:"semantic_criterion_counts"`
	ReviewerKinds           []string       `json:"reviewer_kinds"`
	OutputSHA256            string         `json:"output_sha256,omitempty"`
}

type CampaignReport struct {
	FormatVersion   string                  `json:"format_version"`
	Campaign        CampaignProvenance      `json:"campaign"`
	Status          CampaignStatus          `json:"status"`
	Phases          []CampaignPhaseReport   `json:"phases"`
	Cases           []CampaignCaseTrial     `json:"case_trials"`
	Offline         CampaignOfflineReport   `json:"offline"`
	Budget          CampaignBudgetEvidence  `json:"budget"`
	Recovery        *CampaignRecoveryReport `json:"recovery,omitempty"`
	ReleaseApproved bool                    `json:"release_approved"`
	Warnings        []string                `json:"warnings"`
}

// ReadCampaignReportSpec combines a versioned protocol with a local evidence
// manifest. Relative run/review paths are resolved from the evidence file.
func ReadCampaignReportSpec(dataset *Dataset, protocolPath, evidencePath string) (CampaignReportSpec, error) {
	var spec CampaignReportSpec
	protocolData, err := readBoundedFile(protocolPath, maxCampaignSpecBytes)
	if err != nil {
		return spec, err
	}
	var protocol campaignProtocol
	if err = decodeCampaignJSON(protocolData, &protocol); err != nil {
		return spec, fmt.Errorf("decode campaign protocol: %w", err)
	}
	evidenceData, err := readBoundedFile(evidencePath, maxCampaignSpecBytes)
	if err != nil {
		return spec, err
	}
	var evidence CampaignEvidence
	if err = decodeCampaignJSON(evidenceData, &evidence); err != nil {
		return spec, fmt.Errorf("decode campaign evidence: %w", err)
	}
	protocolHash := digest(protocolData)
	if err = validateCampaignProtocol(dataset, protocol, protocolHash, evidence); err != nil {
		return spec, err
	}

	models := make([]string, 0, len(protocol.Models))
	for _, model := range protocol.Models {
		models = append(models, strings.TrimSpace(model.RequestedModel))
	}
	evidenceByPhase := make(map[string]CampaignEvidencePhase, len(evidence.Phases))
	for _, phase := range evidence.Phases {
		evidenceByPhase[phase.Name] = phase
	}
	definitions := []struct {
		name        string
		phase       campaignProtocolPhase
		metamorphic bool
	}{{"smoke", protocol.Execution.Smoke, false}, {"primary", protocol.Execution.Primary, protocol.Execution.Metamorphic.Enabled}}
	for _, definition := range definitions {
		observed := evidenceByPhase[definition.name]
		runsByModel := make(map[string]CampaignRunSpec, len(observed.Runs))
		for _, run := range observed.Runs {
			run.RunDirectory = resolveEvidencePath(evidencePath, run.RunDirectory)
			if run.SemanticReviews != "" {
				run.SemanticReviews = resolveEvidencePath(evidencePath, run.SemanticReviews)
			}
			if run.RecoveryEvidence != "" {
				run.RecoveryEvidence = resolveEvidencePath(evidencePath, run.RecoveryEvidence)
			}
			if run.RecoverySemanticReviews != "" {
				run.RecoverySemanticReviews = resolveEvidencePath(evidencePath, run.RecoverySemanticReviews)
			}
			runsByModel[run.RequestedModel] = run
		}
		phase := CampaignPhaseSpec{Name: definition.name, Suite: definition.phase.Suite, Trials: definition.phase.TrialsPerCase, Metamorphic: definition.metamorphic}
		for _, model := range models {
			phase.Runs = append(phase.Runs, runsByModel[model])
		}
		spec.Phases = append(spec.Phases, phase)
	}
	spec.FormatVersion = campaignReportVersion
	spec.CampaignID = protocol.CampaignID
	spec.DatasetVersion = protocol.DatasetVersion
	spec.DatasetManifestSHA256 = protocol.DatasetManifestSHA256
	spec.SourceCommit = evidence.ExecutionSourceCommit
	spec.PricingVerifiedOn = evidence.PricingVerifiedOn
	spec.BaselineFilesSHA256 = maps.Clone(evidence.BaselineFilesSHA256)
	spec.Budget = evidence.Budget
	spec.Execution = CampaignExecutionSpec{
		MaxRetries: protocol.Execution.MaxRetries,
		Limits: ExecutionLimits{
			Concurrency:     protocol.Execution.Concurrency,
			AttemptTimeout:  time.Duration(protocol.Execution.AttemptTimeoutSeconds) * time.Second,
			GlobalTimeout:   time.Duration(protocol.Execution.GlobalTimeoutSeconds) * time.Second,
			MinInterval:     time.Duration(protocol.Execution.MinIntervalSeconds) * time.Second,
			MaxOutputTokens: int32(protocol.Execution.MaxOutputTokens),
		},
	}
	spec.ProtocolSHA256 = protocolHash
	return spec, nil
}

// ReadCampaignExecutionConfig verifies the frozen protocol before any provider
// interaction and returns every declared phase/model plan.
func ReadCampaignExecutionConfig(dataset *Dataset, protocolPath string) (CampaignExecutionConfig, error) {
	var result CampaignExecutionConfig
	data, err := readBoundedFile(protocolPath, maxCampaignSpecBytes)
	if err != nil {
		return result, err
	}
	var protocol campaignProtocol
	if err = decodeCampaignJSON(data, &protocol); err != nil {
		return result, fmt.Errorf("decode campaign protocol: %w", err)
	}
	if err = validateCampaignProtocolDefinition(dataset, protocol); err != nil {
		return result, err
	}
	prices, source, err := campaignProtocolPrices(protocol)
	if err != nil {
		return result, err
	}
	result = CampaignExecutionConfig{
		CampaignID: protocol.CampaignID, ProtocolSHA256: digest(data),
		DatasetVersion: protocol.DatasetVersion, DatasetManifestSHA256: protocol.DatasetManifestSHA256,
		Execution: CampaignExecutionSpec{MaxRetries: protocol.Execution.MaxRetries, Limits: ExecutionLimits{
			Concurrency: protocol.Execution.Concurrency, AttemptTimeout: time.Duration(protocol.Execution.AttemptTimeoutSeconds) * time.Second,
			GlobalTimeout: time.Duration(protocol.Execution.GlobalTimeoutSeconds) * time.Second, MinInterval: time.Duration(protocol.Execution.MinIntervalSeconds) * time.Second,
			MaxOutputTokens: int32(protocol.Execution.MaxOutputTokens),
		}},
		MaximumGenerationCalls: protocol.Execution.MaximumProviderCalls,
		HardCeilingUSD:         protocol.Budget.HardCeiling, PricingSource: source, Prices: prices,
		BaselineSourceCommit: protocol.Solution.BaselineSourceCommit,
		BaselineFiles:        maps.Clone(protocol.Solution.BaselineFiles),
		ParentCampaignID:     strings.TrimSpace(protocol.Solution.ParentCampaignID),
		ChangeDescription:    strings.TrimSpace(protocol.Solution.ChangeDescription),
		Phases:               []CampaignExecutionPhase{{Name: "smoke", Suite: protocol.Execution.Smoke.Suite, Trials: protocol.Execution.Smoke.TrialsPerCase}, {Name: "primary", Suite: protocol.Execution.Primary.Suite, Trials: protocol.Execution.Primary.TrialsPerCase, Metamorphic: protocol.Execution.Metamorphic.Enabled}},
	}
	for _, model := range protocol.Models {
		result.Models = append(result.Models, strings.TrimSpace(model.RequestedModel))
	}
	return result, nil
}

func decodeCampaignJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing JSON data", ErrInvalidCampaignReport)
	}
	return nil
}

func resolveEvidencePath(evidencePath, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(evidencePath), target))
}

func validateCampaignProtocol(dataset *Dataset, protocol campaignProtocol, protocolHash string, evidence CampaignEvidence) error {
	if err := validateCampaignProtocolDefinition(dataset, protocol); err != nil {
		return err
	}
	if evidence.FormatVersion != campaignReportVersion || evidence.CampaignID != protocol.CampaignID || evidence.ProtocolSHA256 != protocolHash || evidence.DatasetManifestSHA256 != protocol.DatasetManifestSHA256 || strings.TrimSpace(evidence.ExecutionSourceCommit) == "" {
		return fmt.Errorf("%w: evidence does not bind the protocol, dataset and execution commit", ErrInvalidCampaignReport)
	}
	if _, err := time.Parse(time.DateOnly, evidence.PricingVerifiedOn); err != nil || !maps.Equal(evidence.BaselineFilesSHA256, protocol.Solution.BaselineFiles) {
		return fmt.Errorf("%w: pricing date or baseline file attestation mismatch", ErrInvalidCampaignReport)
	}
	expectedCountRequests := (protocol.Execution.Smoke.Cases + protocol.Execution.Primary.Cases) * len(protocol.Models)
	if err := validateCampaignBudgetEvidence(protocol, evidence.Budget, expectedCountRequests); err != nil {
		return err
	}
	models := map[string]bool{}
	for _, item := range protocol.Models {
		models[strings.TrimSpace(item.RequestedModel)] = true
	}
	phases := map[string]campaignProtocolPhase{"smoke": protocol.Execution.Smoke, "primary": protocol.Execution.Primary}
	if len(evidence.Phases) != len(phases) {
		return fmt.Errorf("%w: evidence must declare every protocol phase", ErrInvalidCampaignReport)
	}
	seenPhases := map[string]bool{}
	for _, phase := range evidence.Phases {
		if _, ok := phases[phase.Name]; !ok || seenPhases[phase.Name] {
			return fmt.Errorf("%w: duplicate or unknown evidence phase", ErrInvalidCampaignReport)
		}
		seenPhases[phase.Name] = true
		if len(phase.Runs) != len(models) {
			return fmt.Errorf("%w: evidence phase must declare every model", ErrInvalidCampaignReport)
		}
		seenModels := map[string]bool{}
		for _, run := range phase.Runs {
			if !models[run.RequestedModel] || seenModels[run.RequestedModel] || strings.TrimSpace(run.RunDirectory) == "" {
				return fmt.Errorf("%w: duplicate, unknown or incomplete evidence model", ErrInvalidCampaignReport)
			}
			seenModels[run.RequestedModel] = true
		}
	}
	return nil
}

func validateCampaignBudgetEvidence(protocol campaignProtocol, budget CampaignBudgetEvidence, expectedCountRequests int) error {
	invalid := budget.PricingSource != campaignPricingSource(protocol) ||
		budget.HardCeilingUSD != protocol.Budget.HardCeiling ||
		budget.CountTokenRequests != expectedCountRequests ||
		budget.ExpectedGenerationCalls != protocol.Execution.MaximumProviderCalls ||
		budget.PreflightInputTokens < 0 || budget.PreflightInputTokens > budget.ReservedInputTokens ||
		budget.ReservedInputTokens != int64(budget.ExpectedGenerationCalls)*CampaignBudgetMaxInputTokens ||
		budget.ReservedOutputTokens != int64(budget.ExpectedGenerationCalls)*int64(protocol.Execution.MaxOutputTokens) ||
		!finite(budget.ReservedInputUSD) || budget.ReservedInputUSD < 0 ||
		!finite(budget.ReservedOutputUSD) || budget.ReservedOutputUSD < 0 ||
		!finite(budget.ReservedTotalUSD) || budget.ReservedTotalUSD < 0 || budget.ReservedTotalUSD > budget.HardCeilingUSD ||
		math.Abs(budget.ReservedInputUSD+budget.ReservedOutputUSD-budget.ReservedTotalUSD) > 1e-9 ||
		!finite(budget.PreflightHeadroomUSD) || math.Abs(budget.HardCeilingUSD-budget.ReservedTotalUSD-budget.PreflightHeadroomUSD) > 1e-9 ||
		!finite(budget.AccountedSpendUSD) || budget.AccountedSpendUSD < 0 || budget.AccountedSpendUSD > budget.HardCeilingUSD ||
		budget.ObservedGenerationCalls < 0 || budget.ObservedGenerationCalls > budget.ExpectedGenerationCalls ||
		budget.GuardRecordedUsageCalls < 0 || budget.GuardRecordedUsageCalls > budget.ObservedGenerationCalls ||
		budget.ProviderUsageComplete && budget.GuardRecordedUsageCalls != budget.ObservedGenerationCalls
	if invalid {
		return fmt.Errorf("%w: budget evidence is inconsistent", ErrInvalidCampaignReport)
	}
	return nil
}

func validateCampaignProtocolDefinition(dataset *Dataset, protocol campaignProtocol) error {
	if dataset == nil {
		return fmt.Errorf("%w: dataset is required", ErrInvalidCampaignReport)
	}
	if strings.TrimSpace(protocol.ProtocolVersion) == "" || strings.TrimSpace(protocol.CampaignID) == "" || protocol.Scope != "development" || protocol.AllowHoldout {
		return fmt.Errorf("%w: protocol identity or development-only scope is invalid", ErrInvalidCampaignReport)
	}
	if protocol.DatasetVersion != dataset.Version || protocol.DatasetManifestSHA256 != dataset.ManifestSHA256 {
		return fmt.Errorf("%w: protocol dataset mismatch", ErrInvalidCampaignReport)
	}
	if strings.TrimSpace(protocol.Solution.BaselineSourceCommit) == "" || len(protocol.Solution.BaselineFiles) == 0 || !protocol.Solution.RequireCleanTreeBeforeLive {
		return fmt.Errorf("%w: baseline solution declaration is incomplete", ErrInvalidCampaignReport)
	}
	changedSolution := protocol.Solution.PromptChangeAllowed || protocol.Solution.GenerationConfigChangeAllowed || protocol.Solution.QualityFixesAllowed
	if changedSolution && (strings.TrimSpace(protocol.Solution.ParentCampaignID) == "" || strings.TrimSpace(protocol.Solution.ChangeDescription) == "") {
		return fmt.Errorf("%w: changed solution must identify its parent campaign and change description", ErrInvalidCampaignReport)
	}
	for path, hash := range protocol.Solution.BaselineFiles {
		if !filepath.IsLocal(path) || len(hash) != 64 {
			return fmt.Errorf("%w: invalid baseline file attestation", ErrInvalidCampaignReport)
		}
	}
	if protocol.Execution.Concurrency != 1 || protocol.Execution.MaxRetries != 0 || protocol.Execution.RetryForQuality || protocol.Execution.AttemptTimeoutSeconds <= 0 || protocol.Execution.GlobalTimeoutSeconds <= 0 || protocol.Execution.MinIntervalSeconds <= 0 || protocol.Execution.MaxOutputTokens != CampaignBudgetMaxOutputTokens {
		return fmt.Errorf("%w: invalid bounded execution settings", ErrInvalidCampaignReport)
	}
	if len(protocol.Models) == 0 {
		return fmt.Errorf("%w: protocol models are required", ErrInvalidCampaignReport)
	}
	models := map[string]bool{}
	for _, item := range protocol.Models {
		model := strings.TrimSpace(item.RequestedModel)
		if model == "" || models[model] {
			return fmt.Errorf("%w: duplicate or empty protocol model", ErrInvalidCampaignReport)
		}
		models[model] = true
	}
	phases := map[string]campaignProtocolPhase{"smoke": protocol.Execution.Smoke, "primary": protocol.Execution.Primary}
	totalCalls := 0
	for name, phase := range phases {
		if phase.Suite != map[string]string{"smoke": "smoke", "primary": "development"}[name] || phase.Cases != len(dataset.Suites[phase.Suite]) || phase.TrialsPerCase <= 0 || phase.CallsTotal != phase.Cases*phase.TrialsPerCase*len(models) {
			return fmt.Errorf("%w: %s phase denominators are inconsistent", ErrInvalidCampaignReport, name)
		}
		if name == "primary" && protocol.ExpectedTrials != phase.TrialsPerCase {
			return fmt.Errorf("%w: primary trials differ from expected_trials", ErrInvalidCampaignReport)
		}
		totalCalls += phase.CallsTotal
	}
	if protocol.Execution.MaximumProviderCalls != totalCalls {
		return fmt.Errorf("%w: provider call ceiling differs from phase totals", ErrInvalidCampaignReport)
	}
	if totalCalls <= 0 || totalCalls > CampaignBudgetMaxRequests {
		return fmt.Errorf("%w: provider call count exceeds campaign ceiling", ErrInvalidCampaignReport)
	}
	if protocol.Budget.Currency != "USD" || protocol.Budget.HardCeiling != CampaignBudgetCeilingUSD || !protocol.Budget.GuardRequiredBeforeLive || protocol.Budget.MaxOutputTokensPerRequest != int64(protocol.Execution.MaxOutputTokens) {
		return fmt.Errorf("%w: campaign budget declaration is inconsistent", ErrInvalidCampaignReport)
	}
	if _, _, err := campaignProtocolPrices(protocol); err != nil {
		return err
	}
	return nil
}

func campaignProtocolPrices(protocol campaignProtocol) ([]CampaignPrice, string, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(protocol.Budget.Pricing, &values); err != nil {
		return nil, "", fmt.Errorf("%w: decode campaign pricing: %v", ErrInvalidCampaignReport, err)
	}
	var source string
	if err := json.Unmarshal(values["source"], &source); err != nil || strings.TrimSpace(source) == "" {
		return nil, "", fmt.Errorf("%w: pricing source is required", ErrInvalidCampaignReport)
	}
	var verify bool
	if err := json.Unmarshal(values["must_verify_on_execution_date"], &verify); err != nil || !verify {
		return nil, "", fmt.Errorf("%w: pricing must be verified on execution date", ErrInvalidCampaignReport)
	}
	delete(values, "source")
	delete(values, "must_verify_on_execution_date")
	prices := make([]CampaignPrice, 0, len(protocol.Models))
	for _, item := range protocol.Models {
		model := strings.TrimSpace(item.RequestedModel)
		raw, ok := values[model]
		if !ok {
			return nil, "", fmt.Errorf("%w: missing protocol price for %s", ErrInvalidCampaignReport, model)
		}
		var price campaignProtocolPrice
		if err := decodeCampaignJSON(raw, &price); err != nil || price.InputUSDPerMillionTokens <= 0 || price.OutputUSDPerMillionTokens <= 0 {
			return nil, "", fmt.Errorf("%w: invalid protocol price for %s", ErrInvalidCampaignReport, model)
		}
		prices = append(prices, CampaignPrice{Model: model, InputUSDPerMillion: price.InputUSDPerMillionTokens, OutputUSDPerMillion: price.OutputUSDPerMillionTokens})
		delete(values, model)
	}
	if len(values) != 0 {
		return nil, "", fmt.Errorf("%w: undeclared model pricing", ErrInvalidCampaignReport)
	}
	return prices, source, nil
}

func campaignPricingSource(protocol campaignProtocol) string {
	var values map[string]json.RawMessage
	if json.Unmarshal(protocol.Budget.Pricing, &values) != nil {
		return ""
	}
	var source string
	_ = json.Unmarshal(values["source"], &source)
	return source
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	if err = errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: file exceeds size limit", ErrInvalidCampaignReport)
	}
	return data, nil
}

// BuildCampaignReport verifies immutable run journals and optional semantic
// review documents before aggregation. Operational failures are evidence, not
// invalid reports; missing or conflicting provenance is rejected fail-closed.
func BuildCampaignReport(ctx context.Context, dataset *Dataset, spec CampaignReportSpec) (CampaignReport, error) {
	report := CampaignReport{
		FormatVersion: campaignReportVersion,
		Campaign: CampaignProvenance{
			ID: spec.CampaignID, DatasetVersion: spec.DatasetVersion,
			DatasetManifestSHA256: spec.DatasetManifestSHA256,
			SourceCommit:          spec.SourceCommit, ProtocolSHA256: spec.ProtocolSHA256,
			PricingVerifiedOn: spec.PricingVerifiedOn,
		},
		Budget: spec.Budget,
		Warnings: []string{
			"Repeated trials are measurements of the same fixed cases, not independent population samples.",
			"Deterministic/schema success and semantic assessment are reported separately.",
			"Reviewer identity is self-declared; agent review is not human certification and no release approval is inferred.",
			"Cost remains null unless verified pricing and complete usage evidence are configured.",
		},
	}
	if err := validateCampaignSpec(dataset, spec); err != nil {
		return report, err
	}
	if ctx == nil {
		return report, fmt.Errorf("%w: context is required", ErrInvalidCampaignReport)
	}

	seenRuns := map[string]bool{}
	for _, phaseSpec := range spec.Phases {
		phase := CampaignPhaseReport{Name: phaseSpec.Name, Suite: phaseSpec.Suite, Trials: phaseSpec.Trials, Metamorphic: phaseSpec.Metamorphic}
		for _, runSpec := range phaseSpec.Runs {
			if seenRuns[runSpec.RunDirectory] {
				return report, fmt.Errorf("%w: duplicate run directory", ErrInvalidCampaignReport)
			}
			seenRuns[runSpec.RunDirectory] = true
			model, cases, err := buildCampaignModel(dataset, spec, phaseSpec, runSpec)
			if err != nil {
				return report, err
			}
			phase.Models = append(phase.Models, model)
			if model.Recovery != nil {
				if report.Recovery == nil {
					report.Recovery = &CampaignRecoveryReport{OriginalReservedUpperUSD: spec.Budget.ReservedTotalUSD, OriginalAccountedUSD: spec.Budget.AccountedSpendUSD, HardCeilingUSD: spec.Budget.HardCeilingUSD, ProviderUsageComplete: spec.Budget.ProviderUsageComplete}
				}
				report.Recovery.EvidenceDocuments++
				report.Recovery.TargetSlots += model.Recovery.TargetSlots
				report.Recovery.RecoveryAttempts += model.Recovery.RecoveryAttempts
				report.Recovery.RecoveredSlots += model.Recovery.RecoveredSlots
				report.Recovery.MissingSlots += len(model.Recovery.MissingSlots)
				report.Recovery.ReservedAdditionalCalls += model.Recovery.ReservedAdditionalCalls
				report.Recovery.AdditionalUpperBoundUSD += model.Recovery.AdditionalUpperBoundUSD
				report.Recovery.AdditionalAccountedUSD += model.Recovery.AccountedAdditionalUSD
				report.Recovery.ProviderUsageComplete = report.Recovery.ProviderUsageComplete && model.Recovery.ProviderUsageComplete
			}
			mergeCampaignCoverage(&phase.Coverage, model.Coverage)
			report.Cases = append(report.Cases, cases...)
		}
		finalizeCampaignCoverage(&phase.Coverage)
		report.Phases = append(report.Phases, phase)
		mergeCampaignCoverage(&report.Status.CampaignCoverage, phase.Coverage)
		report.Status.Models += len(phase.Models)
	}
	report.Status.Phases = len(report.Phases)
	finalizeCampaignCoverage(&report.Status.CampaignCoverage)
	if report.Recovery != nil {
		report.Recovery.CombinedUpperBoundUSD = report.Recovery.OriginalReservedUpperUSD + report.Recovery.AdditionalUpperBoundUSD
		report.Recovery.CombinedAccountedUSD = report.Recovery.OriginalAccountedUSD + report.Recovery.AdditionalAccountedUSD
		report.Recovery.WithinHardCeiling = report.Recovery.CombinedUpperBoundUSD <= report.Recovery.HardCeilingUSD+1e-9
		if !report.Recovery.WithinHardCeiling || report.Recovery.CombinedAccountedUSD > report.Recovery.HardCeilingUSD+1e-9 {
			return report, fmt.Errorf("%w: original plus recovery upper bound exceeds campaign ceiling", ErrInvalidCampaignReport)
		}
	}
	actualRequests := 0
	for _, phase := range report.Phases {
		for _, model := range phase.Models {
			actualRequests += model.Operations.Requests
		}
	}
	if report.Status.ExpectedSlots != spec.Budget.ExpectedGenerationCalls || actualRequests != spec.Budget.ObservedGenerationCalls {
		return report, fmt.Errorf("%w: budget denominators do not match aggregated runs", ErrInvalidCampaignReport)
	}
	offline, err := buildCampaignOfflineReport(ctx, dataset)
	if err != nil {
		return report, err
	}
	report.Offline = offline
	return report, nil
}

func validateCampaignSpec(dataset *Dataset, spec CampaignReportSpec) error {
	if dataset == nil {
		return fmt.Errorf("%w: dataset is required", ErrInvalidCampaignReport)
	}
	if spec.FormatVersion != campaignReportVersion || strings.TrimSpace(spec.CampaignID) == "" || strings.TrimSpace(spec.SourceCommit) == "" || strings.TrimSpace(spec.ProtocolSHA256) == "" {
		return fmt.Errorf("%w: campaign identity, protocol hash and source commit are required", ErrInvalidCampaignReport)
	}
	if strings.TrimSpace(spec.PricingVerifiedOn) == "" || len(spec.BaselineFilesSHA256) == 0 || spec.Budget.ExpectedGenerationCalls <= 0 || spec.Budget.HardCeilingUSD != CampaignBudgetCeilingUSD || spec.Budget.ReservedTotalUSD > spec.Budget.HardCeilingUSD || spec.Budget.AccountedSpendUSD > spec.Budget.HardCeilingUSD {
		return fmt.Errorf("%w: execution attestations are incomplete or exceed budget", ErrInvalidCampaignReport)
	}
	if spec.DatasetVersion != dataset.Version || spec.DatasetManifestSHA256 != dataset.ManifestSHA256 {
		return fmt.Errorf("%w: dataset provenance mismatch", ErrInvalidCampaignReport)
	}
	if len(spec.Phases) == 0 {
		return fmt.Errorf("%w: at least one phase is required", ErrInvalidCampaignReport)
	}
	if spec.Execution.MaxRetries < 0 || spec.Execution.Limits.Validate() != nil {
		return fmt.Errorf("%w: declared execution limits are invalid", ErrInvalidCampaignReport)
	}
	seenPhases := map[string]bool{}
	for _, phase := range spec.Phases {
		if strings.TrimSpace(phase.Name) == "" || seenPhases[phase.Name] {
			return fmt.Errorf("%w: duplicate or empty phase", ErrInvalidCampaignReport)
		}
		seenPhases[phase.Name] = true
		if phase.Suite == "holdout" || phase.Suite == "critical_all" {
			return fmt.Errorf("%w: holdout use is forbidden in campaign reports", ErrInvalidCampaignReport)
		}
		if phase.Suite != "smoke" && phase.Suite != "development" {
			return fmt.Errorf("%w: phase suite must be smoke or development", ErrInvalidCampaignReport)
		}
		if phase.Trials <= 0 || len(phase.Runs) == 0 {
			return fmt.Errorf("%w: positive trials and declared model runs are required", ErrInvalidCampaignReport)
		}
		seenModels := map[string]bool{}
		for _, run := range phase.Runs {
			model := strings.TrimSpace(run.RequestedModel)
			if model == "" || strings.TrimSpace(run.RunDirectory) == "" {
				return fmt.Errorf("%w: model and run directory are required", ErrInvalidCampaignReport)
			}
			if seenModels[model] {
				return fmt.Errorf("%w: duplicate requested model in phase", ErrInvalidCampaignReport)
			}
			seenModels[model] = true
		}
	}
	return nil
}

func loadCampaignRecoveryOverlay(dataset *Dataset, spec CampaignReportSpec, runSpec CampaignRunSpec) (campaignRecoveryOverlay, error) {
	overlay := campaignRecoveryOverlay{
		slots:               map[string]RecoveredAttempt{},
		reviewsByCriterion:  map[string]SemanticReview{},
		reviewerKindsBySlot: map[string]map[string]bool{},
	}
	if runSpec.RecoveryEvidence == "" {
		if runSpec.RecoverySemanticReviews != "" {
			return overlay, fmt.Errorf("%w: recovery reviews require recovery evidence", ErrInvalidCampaignReport)
		}
		return overlay, nil
	}
	data, err := readBoundedFile(runSpec.RecoveryEvidence, 64*1024*1024)
	if err != nil {
		return overlay, fmt.Errorf("read campaign recovery evidence: %w", err)
	}
	evidence, err := ReadCampaignRecovery(runSpec.RecoveryEvidence)
	if err != nil {
		return overlay, err
	}
	expectedPriorSpend := spec.Budget.AccountedSpendUSD
	if !spec.Budget.ProviderUsageComplete {
		expectedPriorSpend = spec.Budget.ReservedTotalUSD
	}
	if evidence.Manifest.BaseProtocolSHA256 != spec.ProtocolSHA256 ||
		!maps.Equal(evidence.Manifest.Binding.BaselineFilesSHA256, spec.BaselineFilesSHA256) ||
		evidence.Manifest.Budget.HardCeilingUSD != spec.Budget.HardCeilingUSD ||
		!evidence.Manifest.Budget.PriorRequestsKnown ||
		evidence.Manifest.Budget.PriorRequests != spec.Budget.ObservedGenerationCalls ||
		evidence.Manifest.Budget.PriorSpentUSD == nil ||
		math.Abs(*evidence.Manifest.Budget.PriorSpentUSD-expectedPriorSpend) > 1e-9 {
		return overlay, fmt.Errorf("%w: recovery protocol, baseline or prior budget binding mismatch", ErrInvalidCampaignReport)
	}
	overlay.slots, err = ApplyCampaignRecovery(dataset, runSpec.RunDirectory, evidence)
	if err != nil {
		return overlay, err
	}
	overlay.evidence = evidence
	overlay.evidenceSHA256 = digest(data)
	if runSpec.RecoverySemanticReviews == "" {
		return overlay, nil
	}
	reviewData, err := readBoundedFile(runSpec.RecoverySemanticReviews, maxSemanticReviewBytes)
	if err != nil {
		return overlay, fmt.Errorf("read recovery semantic review evidence: %w", err)
	}
	var document RecoverySemanticReviewDocument
	if err = decodeCampaignJSON(reviewData, &document); err != nil {
		return overlay, fmt.Errorf("decode recovery semantic review evidence: %w", err)
	}
	reviewed, err := ApplyRecoverySemanticReviews(dataset, runSpec.RunDirectory, evidence, document)
	if err != nil {
		return overlay, err
	}
	overlay.semanticReviewSHA256 = digest(reviewData)
	overlay.semanticPendingHuman = reviewed.PendingHumanChecks
	for _, review := range reviewed.Reviews {
		overlay.reviewsByCriterion[semanticReviewKey(review)] = review
		if review.ReviewerKind == "" {
			continue
		}
		key := attemptKey(review.CaseID, review.Trial)
		if overlay.reviewerKindsBySlot[key] == nil {
			overlay.reviewerKindsBySlot[key] = map[string]bool{}
		}
		overlay.reviewerKindsBySlot[key][review.ReviewerKind] = true
	}
	return overlay, nil
}

func campaignRecoveryAccounted(evidence CampaignRecoveryEvidence) (float64, bool, error) {
	budget := evidence.Manifest.Budget
	complete := true
	var total float64
	for _, item := range evidence.Attempts {
		if item.Attempt.RequestCount == 0 {
			continue
		}
		input, output, err := observedUsage(item.Attempt.ProviderResponse)
		if err != nil {
			complete = false
			input = budget.MaxInputTokensPerCall
			output = budget.MaxOutputTokensPerCall
		}
		total += float64(input)*budget.InputUSDPerMillion/1_000_000 + float64(output)*budget.OutputUSDPerMillion/1_000_000
	}
	if !finite(total) || total < 0 || total > evidence.Manifest.Reservation.AdditionalUpperUSD+1e-9 {
		return 0, false, fmt.Errorf("%w: recovery accounted spend exceeds its reservation", ErrInvalidCampaignReport)
	}
	return total, complete, nil
}

func buildCampaignModel(dataset *Dataset, spec CampaignReportSpec, phase CampaignPhaseSpec, runSpec CampaignRunSpec) (CampaignModelReport, []CampaignCaseTrial, error) {
	model := CampaignModelReport{Tasks: map[string]CampaignTaskReport{}}
	record, replay, err := Replay(dataset, runSpec.RunDirectory)
	if err != nil {
		return model, nil, fmt.Errorf("verify campaign run %q/%q: %w", phase.Name, runSpec.RequestedModel, err)
	}
	if record.Commit != spec.SourceCommit {
		return model, nil, fmt.Errorf("%w: source commit mismatch for %s", ErrInvalidCampaignReport, runSpec.RequestedModel)
	}
	if record.Plan.Suite != phase.Suite || record.Plan.Trials != phase.Trials || record.Plan.Metamorphic != phase.Metamorphic {
		return model, nil, fmt.Errorf("%w: suite, trials or metamorphic plan mismatch", ErrInvalidCampaignReport)
	}
	if record.Plan.MaxRetries != spec.Execution.MaxRetries || record.Limits != spec.Execution.Limits {
		return model, nil, fmt.Errorf("%w: run retry or execution limits differ from protocol", ErrInvalidCampaignReport)
	}
	expectedRequests := len(record.Plan.Cases) * record.Plan.Trials * (record.Plan.MaxRetries + 1)
	if record.Plan.MaximumRequests != expectedRequests || record.Plan.RequestLimit != expectedRequests {
		return model, nil, fmt.Errorf("%w: run request denominator differs from complete declared plan", ErrInvalidCampaignReport)
	}
	if record.Plan.Model != strings.TrimSpace(runSpec.RequestedModel) {
		return model, nil, fmt.Errorf("%w: requested model mismatch", ErrInvalidCampaignReport)
	}
	if record.Plan.UsesHoldout || record.Plan.HoldoutAuthorized {
		return model, nil, fmt.Errorf("%w: holdout evidence is forbidden", ErrInvalidCampaignReport)
	}
	if len(record.Plan.SelectedCaseIDs) > 0 {
		return model, nil, fmt.Errorf("%w: phase must cover its complete declared suite", ErrInvalidCampaignReport)
	}

	observed, attempts, err := ReadRun(runSpec.RunDirectory)
	if err != nil || observed.AttemptsSHA256 != record.AttemptsSHA256 {
		return model, nil, fmt.Errorf("%w: run changed during campaign aggregation", ErrInvalidCampaignReport)
	}
	evaluations := replay.Attempts
	semantic := CampaignSemanticSummary{ReviewSource: "none", CriterionCounts: map[string]int{"pass": 0, "fail": 0, "unassessed": 0, "not_applicable": 0}}
	var reviews []SemanticReview
	if runSpec.SemanticReviews != "" {
		data, readErr := readBoundedFile(runSpec.SemanticReviews, maxSemanticReviewBytes)
		if readErr != nil {
			return model, nil, fmt.Errorf("read semantic review evidence: %w", readErr)
		}
		document, readErr := ReadSemanticReviews(runSpec.SemanticReviews)
		if readErr != nil {
			return model, nil, readErr
		}
		reviewed, reviewErr := ApplySemanticReviews(dataset, runSpec.RunDirectory, document)
		if reviewErr != nil {
			return model, nil, reviewErr
		}
		evaluations = reviewed.Report.Attempts
		reviews = reviewed.Reviews
		semantic.ReviewSource = "imported"
		semantic.ReviewDocumentSHA256 = digest(data)
		semantic.PendingHumanChecks = reviewed.PendingHumanChecks
	}
	if len(attempts) != len(evaluations) {
		return model, nil, fmt.Errorf("%w: evaluated attempts do not match journal", ErrInvalidCampaignReport)
	}

	baseSummary, err := Summarize(dataset, record.Plan, attempts, evaluations)
	if err != nil {
		return model, nil, err
	}
	model.Operations = baseSummary.Operations
	model.Semantic = semantic
	model.Provenance = campaignRunProvenance(record, attempts)
	overlay, err := loadCampaignRecoveryOverlay(dataset, spec, runSpec)
	if err != nil {
		return model, nil, err
	}
	if runSpec.RecoveryEvidence != "" {
		accounted, usageComplete, accountErr := campaignRecoveryAccounted(overlay.evidence)
		if accountErr != nil {
			return model, nil, accountErr
		}
		model.Recovery = &CampaignRecoverySummary{
			RecoveryID: overlay.evidence.Manifest.RecoveryID, EvidenceSHA256: overlay.evidenceSHA256,
			TargetSlots: len(overlay.evidence.Manifest.Targets), RecoveryAttempts: len(overlay.evidence.Attempts),
			AttemptStatuses: map[string]int{}, ReservedAdditionalCalls: overlay.evidence.Manifest.Reservation.AdditionalCalls,
			AdditionalUpperBoundUSD: overlay.evidence.Manifest.Reservation.AdditionalUpperUSD,
			AccountedAdditionalUSD:  accounted, ProviderUsageComplete: usageComplete,
			SemanticReviewSHA256: overlay.semanticReviewSHA256, SemanticPendingHuman: overlay.semanticPendingHuman,
		}
		allRecoveryKinds := map[string]bool{}
		for _, item := range overlay.evidence.Attempts {
			model.Recovery.AttemptStatuses[item.Attempt.Status]++
		}
		for _, kinds := range overlay.reviewerKindsBySlot {
			for kind := range kinds {
				allRecoveryKinds[kind] = true
			}
		}
		model.Recovery.SemanticReviewerKinds = sortedSet(allRecoveryKinds)
		if overlay.semanticReviewSHA256 != "" {
			if model.Semantic.ReviewSource == "imported" {
				model.Semantic.ReviewSource = "base_and_recovery"
			} else {
				model.Semantic.ReviewSource = "recovery"
			}
		}
	}

	reviewKindsBySlot := map[string]map[string]bool{}
	allReviewKinds := map[string]bool{}
	baseReviewsByCriterion := map[string]SemanticReview{}
	for _, review := range reviews {
		baseReviewsByCriterion[semanticReviewKey(review)] = review
		if review.ReviewerKind == "" {
			continue
		}
		key := attemptKey(review.CaseID, review.Trial)
		if reviewKindsBySlot[key] == nil {
			reviewKindsBySlot[key] = map[string]bool{}
		}
		reviewKindsBySlot[key][review.ReviewerKind] = true
		allReviewKinds[review.ReviewerKind] = true
	}
	for key, kinds := range overlay.reviewerKindsBySlot {
		if reviewKindsBySlot[key] == nil {
			reviewKindsBySlot[key] = map[string]bool{}
		}
		for kind := range kinds {
			reviewKindsBySlot[key][kind] = true
			allReviewKinds[kind] = true
		}
	}
	model.Semantic.ReviewerKinds = sortedSet(allReviewKinds)
	model.Semantic.PendingHumanChecks = 0

	evaluated := make(map[string]EvaluatedAttempt, len(evaluations))
	for _, item := range evaluations {
		evaluated[summaryAttemptKey(item.CaseID, item.Trial, item.Retry)] = item
	}
	baseMetadata := campaignCaseMetadata(dataset)
	baseFor := map[string]string{}
	for _, planned := range record.Plan.Cases {
		baseFor[planned.CaseID] = planned.CaseID
		if planned.Variant != nil {
			baseFor[planned.CaseID] = planned.Variant.BaseCaseID
		}
	}

	type terminalEvidence struct {
		attempt Attempt
		count   int
	}
	terminal := map[string]terminalEvidence{}
	order := []string{}
	for _, attempt := range attempts {
		key := attemptKey(attempt.CaseID, attempt.Trial)
		item, exists := terminal[key]
		if !exists {
			order = append(order, key)
		}
		item.attempt = attempt
		item.count++
		terminal[key] = item
	}
	expectedSlots := len(record.Plan.Cases) * record.Plan.Trials
	if len(terminal) != expectedSlots {
		return model, nil, fmt.Errorf("%w: missing or duplicate terminal slots", ErrInvalidCampaignReport)
	}

	effectiveAttempts := make([]Attempt, 0, expectedSlots)
	effectiveEvaluations := make([]EvaluatedAttempt, 0, expectedSlots)
	effectiveBySlot := make(map[string]Attempt, expectedSlots)
	evaluationBySlot := make(map[string]EvaluatedAttempt, expectedSlots)
	for _, key := range order {
		original := terminal[key].attempt
		effective := original
		item, exists := evaluated[summaryAttemptKey(original.CaseID, original.Trial, original.Retry)]
		if !exists {
			return model, nil, fmt.Errorf("%w: missing terminal evaluation %s", ErrInvalidCampaignReport, key)
		}
		if recovered, ok := overlay.slots[key]; ok {
			if recovered.OriginalAttemptSHA256 != AttemptSHA256(original) {
				return model, nil, fmt.Errorf("%w: recovery original attempt mismatch %s", ErrInvalidCampaignReport, key)
			}
			effective = recovered.Effective
			if effective.Retry > original.Retry {
				item, err = evaluateCampaignRecoveredAttempt(dataset, record.Plan, effective, overlay.reviewsByCriterion)
				if err != nil {
					return model, nil, err
				}
				if effective.Status == "executed" {
					model.Recovery.RecoveredSlots++
				} else {
					model.Recovery.MissingSlots = append(model.Recovery.MissingSlots, key)
				}
			} else if RecoverableNoResponse(original) {
				model.Recovery.MissingSlots = append(model.Recovery.MissingSlots, key)
			}
		}
		effectiveBySlot[key] = effective
		evaluationBySlot[key] = item
		normalized := effective
		normalized.Retry = original.Retry
		normalizedEvaluation := item
		normalizedEvaluation.Retry = original.Retry
		effectiveAttempts = append(effectiveAttempts, normalized)
		effectiveEvaluations = append(effectiveEvaluations, normalizedEvaluation)
	}
	effectiveSummary, err := Summarize(dataset, record.Plan, effectiveAttempts, effectiveEvaluations)
	if err != nil {
		return model, nil, err
	}
	model.Tasks = campaignTaskReports(effectiveSummary)

	cases := make([]CampaignCaseTrial, 0, len(order))
	for _, key := range order {
		terminalItem := terminal[key]
		original := terminalItem.attempt
		attempt := effectiveBySlot[key]
		item := evaluationBySlot[key]
		baseID := baseFor[attempt.CaseID]
		metadata, exists := baseMetadata[baseID]
		if !exists || metadata.Split == "holdout" {
			return model, nil, fmt.Errorf("%w: unknown or holdout case %s", ErrInvalidCampaignReport, baseID)
		}
		semanticStatus, semanticCounts := campaignSemanticStatus(item.Evaluation.SemanticChecks)
		for _, check := range item.Evaluation.SemanticChecks {
			keyReview := semanticReviewKey(SemanticReview{CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, CriterionID: check.ID})
			review, reviewed := baseReviewsByCriterion[keyReview]
			if attempt.Retry > original.Retry {
				review, reviewed = overlay.reviewsByCriterion[keyReview]
			}
			if !reviewed || review.Result == "unassessed" || review.ReviewerKind != "human" {
				model.Semantic.PendingHumanChecks++
			}
		}
		for result, count := range semanticCounts {
			model.Semantic.CriterionCounts[result] += count
		}
		kinds := sortedSet(reviewKindsBySlot[key])
		row := CampaignCaseTrial{
			Phase: phase.Name, RequestedModel: runSpec.RequestedModel,
			CaseID: attempt.CaseID, BaseCaseID: baseID, Task: metadata.Task,
			FamilyID: metadata.FamilyID, Split: metadata.Split, Trial: attempt.Trial,
			TerminalRetry: attempt.Retry, Attempts: terminalItem.count,
			ExecutionStatus: attempt.Status, DeterministicStatus: item.Evaluation.DeterministicStatus,
			OriginalExecutionStatus: original.Status, OriginalAttemptSHA256: AttemptSHA256(original),
			OverallStatus: item.Evaluation.OverallStatus, SemanticStatus: semanticStatus,
			FailureCodes: append([]string(nil), item.Evaluation.Errors...), Metrics: campaignCaseMetrics(dataset, baseID, attempt, item.Evaluation),
			SemanticCounts: semanticCounts, ReviewerKinds: kinds,
		}
		if originalItem, ok := evaluated[summaryAttemptKey(original.CaseID, original.Trial, original.Retry)]; ok {
			row.OriginalFailureCodes = append([]string(nil), originalItem.Evaluation.Errors...)
		}
		if recovered, ok := overlay.slots[key]; ok && len(recovered.RecoveryAttempts) > 0 {
			row.RecoveryAttempts = len(recovered.RecoveryAttempts)
			row.Attempts += row.RecoveryAttempts
			row.RecoveryEvidenceSHA256 = overlay.evidenceSHA256
			for _, target := range overlay.evidence.Manifest.Targets {
				if target.CaseID == row.CaseID && target.Trial == row.Trial {
					row.RecoveryReason = target.Reason
					break
				}
			}
			for _, recoveryAttempt := range recovered.RecoveryAttempts {
				row.RecoveryAttemptStatuses = append(row.RecoveryAttemptStatuses, recoveryAttempt.Attempt.Status)
			}
			row.Recovered = attempt.Status == "executed" && attempt.Retry > original.Retry
		}
		if strings.TrimSpace(attempt.RawOutput) != "" {
			row.OutputSHA256 = digest([]byte(attempt.RawOutput))
		}
		addCampaignSlot(&model.Coverage, row, kinds)
		task := model.Tasks[metadata.Task]
		addCampaignSlot(&task.Coverage, row, kinds)
		model.Tasks[metadata.Task] = task
		cases = append(cases, row)
	}
	finalizeCampaignCoverage(&model.Coverage)
	for name, task := range model.Tasks {
		finalizeCampaignCoverage(&task.Coverage)
		task.TrialVariability = campaignTrialVariability(cases, name)
		model.Tasks[name] = task
	}
	return model, cases, nil
}

func evaluateCampaignRecoveredAttempt(dataset *Dataset, plan *Plan, attempt Attempt, reviews map[string]SemanticReview) (EvaluatedAttempt, error) {
	evaluation, err := evaluatePlannedAttempt(dataset, plan, attempt)
	if err != nil {
		return EvaluatedAttempt{}, fmt.Errorf("evaluate recovered campaign output %s/%d: %w", attempt.CaseID, attempt.Trial, err)
	}
	if attempt.Status != "executed" {
		evaluation.Errors = append(evaluation.Errors, attempt.Status)
		evaluation.DeterministicStatus = "failed"
		evaluation.OverallStatus = "failed"
		return EvaluatedAttempt{CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, ExecutionStatus: attempt.Status, Evaluation: evaluation}, nil
	}
	humanComplete := len(evaluation.SemanticChecks) > 0
	semanticFail := false
	for i := range evaluation.SemanticChecks {
		check := &evaluation.SemanticChecks[i]
		review, exists := reviews[semanticReviewKey(SemanticReview{CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, CriterionID: check.ID})]
		if exists {
			check.Result = review.Result
		}
		if !exists || review.Result == "unassessed" || review.ReviewerKind != "human" {
			humanComplete = false
		}
		if check.Result == "fail" {
			semanticFail = true
		}
	}
	switch {
	case evaluation.DeterministicStatus == "failed" || semanticFail:
		evaluation.OverallStatus = "failed"
	case humanComplete:
		evaluation.OverallStatus = "human_reviewed"
	default:
		evaluation.OverallStatus = "needs_semantic_review"
	}
	evaluation.ReleaseApproved = false
	return EvaluatedAttempt{CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, ExecutionStatus: attempt.Status, Evaluation: evaluation}, nil
}

func campaignRunProvenance(record RunRecord, attempts []Attempt) CampaignRunProvenance {
	result := CampaignRunProvenance{
		RunID: record.RunID, RunStatus: record.Status, StartedOn: record.StartedOn,
		FinishedOn: record.FinishedOn, AttemptsSHA256: record.AttemptsSHA256,
		RequestedModel: record.Plan.Model, RequestLimit: record.Plan.RequestLimit,
		MaximumRequests: record.Plan.MaximumRequests, Limits: record.Limits,
	}
	versions := map[string]bool{}
	prompts := map[string]bool{}
	inputs := map[string]bool{}
	configs := map[string]bool{}
	for _, attempt := range attempts {
		if attempt.PromptSHA256 != "" {
			prompts[attempt.PromptSHA256] = true
		}
		if attempt.InputSHA256 != "" {
			inputs[attempt.InputSHA256] = true
		}
		if len(attempt.GenerationConfig) > 0 {
			configs[digest(attempt.GenerationConfig)] = true
		}
		if attempt.RequestCount <= 0 {
			continue
		}
		var response struct {
			ModelVersion string `json:"modelVersion"`
		}
		if json.Unmarshal(attempt.ProviderResponse, &response) != nil || strings.TrimSpace(response.ModelVersion) == "" {
			result.UnknownResolvedModelRequests += attempt.RequestCount
			continue
		}
		versions[strings.TrimSpace(response.ModelVersion)] = true
	}
	result.ResolvedModelVersions = sortedSet(versions)
	result.PromptSHA256 = sortedSet(prompts)
	result.InputSHA256 = sortedSet(inputs)
	result.GenerationConfigSHA256 = sortedSet(configs)
	return result
}

func campaignTaskReports(summary Summary) map[string]CampaignTaskReport {
	prediagnosis := CampaignTaskReport{Metrics: map[string]MetricSummary{}, TrialVariability: map[string]MetricVariability{}, SingletonMacroF1: summary.SingletonMacroF1, SingletonCases: summary.SingletonBaseCases, SingletonTrials: summary.SingletonTrials}
	for _, name := range []string{"accepted_outcome_accuracy", "category_accuracy_when_required"} {
		prediagnosis.Metrics[name] = summary.Metrics[name]
	}
	ranking := CampaignTaskReport{Metrics: map[string]MetricSummary{}, TrialVariability: map[string]MetricVariability{}}
	for _, name := range rankingMetricNames() {
		ranking.Metrics[name] = summary.Metrics[name]
	}
	return map[string]CampaignTaskReport{"prediagnosis": prediagnosis, "ranking": ranking}
}

func campaignCaseMetrics(dataset *Dataset, baseID string, attempt Attempt, evaluation Evaluation) map[string]any {
	metrics := make(map[string]any, len(evaluation.Metrics)+4)
	for name, value := range evaluation.Metrics {
		metrics[name] = value
	}
	if strings.HasPrefix(baseID, "RK-") {
		if raw, ok := evaluation.Metrics["pairwise"]; ok {
			data, err := json.Marshal(raw)
			if err == nil {
				var pairwise PairwiseMetrics
				if json.Unmarshal(data, &pairwise) == nil {
					metrics["pairwise_satisfaction"] = pairwise.Satisfaction
					metrics["pairwise_coverage"] = pairwise.Coverage
				}
			}
		}
		return metrics
	}
	for _, c := range dataset.PD {
		if c.ID != baseID {
			continue
		}
		accepted := 0.0
		if value, ok := evaluation.Metrics["accepted_outcome"].(bool); ok && value && attempt.Status == "executed" {
			accepted = 1
		}
		metrics["accepted_outcome_accuracy"] = accepted
		var expected pdExpected
		if json.Unmarshal(c.Expected, &expected) != nil || len(expected.Outcomes) != 1 || expected.Outcomes[0] != "professional_required" {
			return metrics
		}
		correct := 0.0
		var output pdOutput
		if attempt.Status == "executed" && json.Unmarshal([]byte(attempt.RawOutput), &output) == nil && !hasSchemaError(evaluation.Errors) && output.Assessment.Action == "replace" && output.Assessment.Category != "" && slices.Contains(expected.CategoryNames, output.Assessment.Category) {
			correct = 1
		}
		metrics["category_accuracy_when_required"] = correct
		return metrics
	}
	return metrics
}

func campaignTrialVariability(cases []CampaignCaseTrial, task string) map[string]MetricVariability {
	metricNames := []string{"accepted_outcome_accuracy", "category_accuracy_when_required"}
	if task == "ranking" {
		metricNames = rankingMetricNames()
	}
	result := make(map[string]MetricVariability, len(metricNames))
	for _, metric := range metricNames {
		values := map[string][]float64{}
		expected := map[string]bool{}
		for _, row := range cases {
			if row.Task != task {
				continue
			}
			if metric != "category_accuracy_when_required" {
				expected[row.CaseID] = true
			}
			value, ok := summaryNumber(row.Metrics[metric])
			if !ok {
				continue
			}
			expected[row.CaseID] = true
			values[row.CaseID] = append(values[row.CaseID], value)
		}
		item := MetricVariability{ExpectedBaseCases: len(expected), EvaluatedBaseCases: len(values)}
		var rangeSum float64
		var maxRange float64
		for _, observations := range values {
			if len(observations) < 2 {
				continue
			}
			item.ComparableBaseCases++
			low, high := observations[0], observations[0]
			for _, observation := range observations[1:] {
				low = min(low, observation)
				high = max(high, observation)
			}
			within := high - low
			if within > 0 {
				item.VariableBaseCases++
			}
			rangeSum += within
			maxRange = max(maxRange, within)
		}
		if item.ComparableBaseCases > 0 {
			mean := rangeSum / float64(item.ComparableBaseCases)
			item.MeanWithinCaseRange = &mean
			item.MaxWithinCaseRange = &maxRange
		}
		result[metric] = item
	}
	return result
}

func buildCampaignOfflineReport(ctx context.Context, dataset *Dataset) (CampaignOfflineReport, error) {
	result := CampaignOfflineReport{ContractCounts: map[string]int{}, RankingPolicies: []CampaignBaselinePolicyReport{}}
	contracts, err := ExecuteContracts(ctx, dataset)
	if err != nil {
		return result, err
	}
	result.Contracts = contracts
	for _, contract := range contracts {
		result.ContractCounts[contract.Status]++
	}
	baselines, err := EvaluateRankingBaselines(dataset, dataset.Suites["development"])
	if err != nil {
		return result, err
	}
	result.RankingBaseCases = baselines.BaseCases
	result.RankingModelRequests = baselines.ModelRequests
	for _, policy := range baselines.Policies {
		item := CampaignBaselinePolicyReport{Policy: policy.Policy, Metrics: policy.Metrics, DeterministicFailedIDs: []string{}}
		for _, c := range policy.Cases {
			if c.Evaluation.DeterministicStatus == "failed" {
				item.DeterministicFailedIDs = append(item.DeterministicFailedIDs, c.CaseID)
			}
			for _, check := range c.Evaluation.SemanticChecks {
				if check.Result == "" || check.Result == "unassessed" {
					item.SemanticUnknownCases++
					break
				}
			}
		}
		slices.Sort(item.DeterministicFailedIDs)
		result.RankingPolicies = append(result.RankingPolicies, item)
	}
	return result, nil
}

func campaignCaseMetadata(dataset *Dataset) map[string]CaseMetadata {
	result := make(map[string]CaseMetadata, len(dataset.PD)+len(dataset.RK))
	for _, c := range dataset.PD {
		result[c.ID] = c.CaseMetadata
	}
	for _, c := range dataset.RK {
		result[c.ID] = c.CaseMetadata
	}
	return result
}

func campaignSemanticStatus(checks []SemanticCheck) (string, map[string]int) {
	counts := map[string]int{"pass": 0, "fail": 0, "unassessed": 0, "not_applicable": 0}
	for _, check := range checks {
		result := check.Result
		if result == "" {
			result = "unassessed"
		}
		counts[result]++
	}
	switch {
	case len(checks) == 0:
		return "not_applicable", counts
	case counts["unassessed"] > 0:
		return "unassessed", counts
	case counts["fail"] > 0:
		return "failed", counts
	default:
		return "assessed", counts
	}
}

func addCampaignSlot(coverage *CampaignCoverage, row CampaignCaseTrial, reviewerKinds []string) {
	coverage.ExpectedSlots++
	coverage.TerminalSlots++
	if row.ExecutionStatus == "executed" {
		coverage.ExecutedSlots++
	} else {
		coverage.ExecutionFailedSlots++
	}
	if row.DeterministicStatus == "passed" {
		coverage.DeterministicPassed++
	} else {
		coverage.DeterministicFailed++
	}
	if row.SemanticStatus == "unassessed" {
		coverage.SemanticUnknownSlots++
	}
	if row.SemanticStatus == "failed" {
		coverage.SemanticFailedSlots++
	}
	if slices.Contains(reviewerKinds, "agent") {
		coverage.AgentReviewedSlots++
	}
	if row.OverallStatus == "human_reviewed" {
		coverage.HumanReviewedSlots++
	}
	if row.Recovered {
		coverage.RecoveredSlots++
	}
}

func mergeCampaignCoverage(target *CampaignCoverage, source CampaignCoverage) {
	target.ExpectedSlots += source.ExpectedSlots
	target.TerminalSlots += source.TerminalSlots
	target.ExecutedSlots += source.ExecutedSlots
	target.ExecutionFailedSlots += source.ExecutionFailedSlots
	target.DeterministicPassed += source.DeterministicPassed
	target.DeterministicFailed += source.DeterministicFailed
	target.SemanticUnknownSlots += source.SemanticUnknownSlots
	target.SemanticFailedSlots += source.SemanticFailedSlots
	target.AgentReviewedSlots += source.AgentReviewedSlots
	target.HumanReviewedSlots += source.HumanReviewedSlots
	target.RecoveredSlots += source.RecoveredSlots
}

func finalizeCampaignCoverage(coverage *CampaignCoverage) {
	coverage.EvidenceComplete = coverage.ExpectedSlots > 0 && coverage.TerminalSlots == coverage.ExpectedSlots
	coverage.ResponsesComplete = coverage.EvidenceComplete && coverage.ExecutedSlots == coverage.ExpectedSlots
	coverage.SemanticsComplete = coverage.EvidenceComplete && coverage.SemanticUnknownSlots == 0
	coverage.HumanReviewComplete = coverage.EvidenceComplete && coverage.HumanReviewedSlots == coverage.ExpectedSlots
}

func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
