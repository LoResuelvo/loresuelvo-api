package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const campaignExportVersion = "3"

var ErrInvalidCampaignExport = errors.New("invalid campaign export")

// CampaignExportSpec binds a longitudinal, versionable export to its exact
// offline evidence and to the committed exporter implementation.
type CampaignExportSpec struct {
	Report               CampaignReportSpec
	ProtocolPath         string
	EvidencePath         string
	RecoveryAddendumPath string
	ExporterSourceCommit string
}

type CampaignExport struct {
	Manifest  CampaignExportManifest
	Attempts  []CampaignExportAttempt
	Responses []CampaignExportResponse
	Results   []CampaignExportResult
	Reviews   []CampaignExportReview `json:"-"`
	Summary   CampaignExportSummary
}

// CampaignExportSummary contains only campaign-level aggregates. Per-slot
// observations live in results.jsonl and are not duplicated here.
type CampaignExportSummary struct {
	FormatVersion   string                  `json:"format_version"`
	Campaign        CampaignProvenance      `json:"campaign"`
	Status          CampaignStatus          `json:"status"`
	Phases          []CampaignPhaseReport   `json:"phases"`
	Offline         CampaignOfflineReport   `json:"offline"`
	Budget          CampaignBudgetEvidence  `json:"budget"`
	Recovery        *CampaignRecoveryReport `json:"recovery,omitempty"`
	ReleaseApproved bool                    `json:"release_approved"`
	Warnings        []string                `json:"warnings"`
}

type CampaignExportManifest struct {
	FormatVersion          string                            `json:"format_version"`
	CampaignID             string                            `json:"campaign_id"`
	DatasetVersion         string                            `json:"dataset_version"`
	DatasetManifestSHA256  string                            `json:"dataset_manifest_sha256"`
	ProtocolSHA256         string                            `json:"protocol_sha256"`
	RecoveryAddendumSHA256 string                            `json:"recovery_addendum_sha256,omitempty"`
	EvidenceManifestSHA256 string                            `json:"evidence_manifest_sha256"`
	ExporterSourceCommit   string                            `json:"exporter_source_commit"`
	BaselineFilesSHA256    map[string]string                 `json:"baseline_files_sha256"`
	Builds                 map[string]*BuildProvenance       `json:"builds_by_sha256"`
	Plans                  map[string]*Plan                  `json:"plans_by_sha256"`
	GenerationConfigs      map[string]json.RawMessage        `json:"generation_configs_by_sha256"`
	SourceCommits          []CampaignExportSourceCommit      `json:"source_commits"`
	Models                 []CampaignExportModelResolution   `json:"models"`
	Counts                 CampaignExportCounts              `json:"counts"`
	Sources                []CampaignExportSource            `json:"sources"`
	Artifacts              map[string]CampaignExportArtifact `json:"artifacts"`
	MissingData            []string                          `json:"missing_data"`
	PrivacyScan            CampaignExportPrivacyScan         `json:"privacy_scan"`
	ReleaseApproved        bool                              `json:"release_approved"`
}

type CampaignExportSourceCommit struct {
	Role   string `json:"role"`
	Commit string `json:"commit"`
}

type CampaignExportModelResolution struct {
	Phase                    string         `json:"phase"`
	RequestedModel           string         `json:"requested_model"`
	ResolvedModelOccurrences map[string]int `json:"resolved_model_occurrences"`
	UnknownAttempts          int            `json:"unknown_attempts"`
}

type CampaignExportCounts struct {
	PlannedSlots       int `json:"planned_slots"`
	ProviderAttempts   int `json:"provider_attempts"`
	Responses          int `json:"responses"`
	MalformedResponses int `json:"malformed_responses"`
	Results            int `json:"results"`
	ReviewRows         int `json:"-"`
	EffectiveReviews   int `json:"effective_criteria"`
	AgentAssessments   int `json:"-"`
	HumanAssessments   int `json:"-"`
	UnassessedReviews  int `json:"unassessed_effective_criteria"`
}

type CampaignExportSource struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Phase   string `json:"phase,omitempty"`
	Model   string `json:"requested_model,omitempty"`
	SHA256  string `json:"sha256"`
	Records int    `json:"records,omitempty"`
}

type CampaignExportArtifact struct {
	SHA256  string `json:"sha256"`
	Records *int   `json:"records"`
}

type CampaignExportPrivacyScan struct {
	Status         string         `json:"status"`
	Method         string         `json:"method"`
	Checks         []string       `json:"checks"`
	Limitations    []string       `json:"limitations"`
	RecordsScanned map[string]int `json:"records_scanned"`
	Findings       map[string]int `json:"findings"`
}

type CampaignExportAttempt struct {
	Provider               string                      `json:"provider"`
	HTTPStatus             *int                        `json:"provider_http_status"`
	ProviderResponseID     *string                     `json:"provider_response_id"`
	FinishReasons          []string                    `json:"finish_reasons"`
	BlockReason            *string                     `json:"block_reason"`
	DurationScope          string                      `json:"duration_scope"`
	AttemptID              string                      `json:"attempt_id"`
	SlotID                 string                      `json:"slot_id"`
	Phase                  string                      `json:"phase"`
	RequestedModel         string                      `json:"requested_model"`
	CaseID                 string                      `json:"case_id"`
	Trial                  int                         `json:"trial"`
	Source                 string                      `json:"source"`
	SourceArtifactID       string                      `json:"source_artifact_id"`
	JournalSequence        *int                        `json:"journal_sequence"`
	Retry                  int                         `json:"retry"`
	RecoveryNumber         *int                        `json:"recovery_number"`
	BaseAttemptSHA256      string                      `json:"base_attempt_sha256,omitempty"`
	Status                 string                      `json:"status"`
	ErrorClass             *string                     `json:"error_class"`
	SanitizedError         *string                     `json:"sanitized_error"`
	StartedOn              *time.Time                  `json:"started_on"`
	LatencyMillis          *int64                      `json:"latency_ms"`
	ProviderRequestCount   int                         `json:"provider_request_count"`
	ProviderHTTPRequestID  *string                     `json:"provider_http_request_id"`
	InputSHA256            string                      `json:"input_sha256,omitempty"`
	PromptSHA256           string                      `json:"prompt_sha256,omitempty"`
	GenerationConfigSHA256 string                      `json:"generation_config_sha256,omitempty"`
	RawOutputSHA256        string                      `json:"raw_output_sha256,omitempty"`
	EffectiveResponseID    *string                     `json:"effective_response_id"`
	ResolvedModel          *string                     `json:"resolved_model"`
	Usage                  CampaignExportProviderUsage `json:"usage"`
	MissingData            []string                    `json:"missing_data"`
}

type CampaignExportProviderUsage struct {
	Source            string                           `json:"source"`
	CachedInputTokens *int64                           `json:"cached_input_tokens"`
	InputTokens       *int64                           `json:"input_tokens"`
	OutputTokens      *int64                           `json:"output_tokens"`
	ThinkingTokens    *int64                           `json:"thinking_tokens"`
	TotalTokens       *int64                           `json:"total_tokens"`
	InputTokenDetails []CampaignExportInputTokenDetail `json:"input_token_details"`
	Missing           map[string]string                `json:"missing"`
}

type CampaignExportInputTokenDetail struct {
	Modality   string `json:"modality"`
	TokenCount *int64 `json:"token_count"`
}

type CampaignExportResponse struct {
	ResponseID          string          `json:"response_id"`
	SlotID              string          `json:"slot_id"`
	Phase               string          `json:"phase"`
	RequestedModel      string          `json:"requested_model"`
	DatasetCaseID       string          `json:"dataset_case_id"`
	Trial               int             `json:"trial"`
	OriginalAttemptID   string          `json:"original_attempt_id"`
	EffectiveAttemptID  string          `json:"effective_attempt_id"`
	Recovered           bool            `json:"recovered"`
	ExecutionStatus     string          `json:"execution_status"`
	FormatStatus        string          `json:"format_status"`
	MissingReason       *string         `json:"missing_reason"`
	InputSHA256         string          `json:"input_sha256,omitempty"`
	PromptSHA256        string          `json:"prompt_sha256,omitempty"`
	GenerationConfigSHA string          `json:"generation_config_sha256,omitempty"`
	OutputSHA256        string          `json:"output_sha256"`
	RawOutput           *string         `json:"raw_output"`
	ParsedOutput        json.RawMessage `json:"parsed_output"`
}

type CampaignExportResult struct {
	OutputSHA256        string                    `json:"output_sha256"`
	SlotID              string                    `json:"slot_id"`
	ResponseID          string                    `json:"response_id"`
	Phase               string                    `json:"phase"`
	RequestedModel      string                    `json:"requested_model"`
	CaseID              string                    `json:"case_id"`
	BaseCaseID          string                    `json:"base_case_id"`
	FamilyID            string                    `json:"family_id"`
	Split               string                    `json:"split"`
	Task                string                    `json:"task"`
	Trial               int                       `json:"trial"`
	OriginalStatus      string                    `json:"original_execution_status"`
	Recovered           bool                      `json:"recovered"`
	RecoveryAttempts    int                       `json:"recovery_attempts"`
	RecoveryReason      string                    `json:"recovery_reason,omitempty"`
	Expected            json.RawMessage           `json:"expected"`
	Observed            json.RawMessage           `json:"observed"`
	ObservedMissing     *string                   `json:"observed_missing_reason"`
	DeterministicStatus string                    `json:"deterministic_status"`
	OverallStatus       string                    `json:"overall_status"`
	SemanticStatus      string                    `json:"semantic_status"`
	FailureCodes        []string                  `json:"failure_codes"`
	Metrics             map[string]any            `json:"metrics"`
	SemanticCounts      map[string]int            `json:"semantic_criterion_counts"`
	ReviewerKinds       []string                  `json:"-"`
	SemanticCriteria    []CampaignExportCriterion `json:"semantic_criteria"`
	ReleaseApproved     bool                      `json:"release_approved"`
}

// CampaignExportCriterion is the public assessment projection. Private reviewer
// identity, evidence prose and reasoning never enter this representation.
type CampaignExportCriterion struct {
	CriterionID     string `json:"criterion_id"`
	CriterionSHA256 string `json:"criterion_sha256"`
	OutputSHA256    string `json:"output_sha256"`
	Severity        string `json:"severity,omitempty"`
	Result          string `json:"result"`
}

type CampaignExportReview struct {
	ReviewID            string     `json:"review_id"`
	SlotID              string     `json:"slot_id"`
	AttemptID           string     `json:"attempt_id"`
	ResponseID          *string    `json:"response_id"`
	Phase               string     `json:"phase"`
	RequestedModel      string     `json:"requested_model"`
	CaseID              string     `json:"case_id"`
	Trial               int        `json:"trial"`
	Retry               int        `json:"retry"`
	Source              string     `json:"source"`
	ReviewFormatVersion string     `json:"-"`
	EffectiveForResult  bool       `json:"effective_for_result"`
	OutputSHA256        string     `json:"output_sha256"`
	CriterionID         string     `json:"criterion_id"`
	CriterionSHA256     string     `json:"criterion_sha256"`
	Criterion           string     `json:"criterion"`
	Severity            string     `json:"severity"`
	Result              string     `json:"result"`
	Evidence            string     `json:"-"`
	EvidenceQuote       string     `json:"-"`
	EvidenceLocation    string     `json:"-"`
	Reason              string     `json:"-"`
	Reviewer            string     `json:"-"`
	ReviewerKind        string     `json:"-"`
	ReviewedOn          *time.Time `json:"-"`
}

// BuildCampaignExport validates all source evidence again and materializes a
// complete, transcript-bearing campaign package without making provider calls.
func BuildCampaignExport(ctx context.Context, dataset *Dataset, spec CampaignExportSpec) (CampaignExport, error) {
	var result CampaignExport
	if ctx == nil || dataset == nil || strings.TrimSpace(spec.ExporterSourceCommit) == "" {
		return result, fmt.Errorf("%w: context, dataset and exporter source commit are required", ErrInvalidCampaignExport)
	}
	protocol, err := readBoundedFile(spec.ProtocolPath, maxCampaignSpecBytes)
	if err != nil {
		return result, fmt.Errorf("read export protocol: %w", err)
	}
	if digest(protocol) != spec.Report.ProtocolSHA256 {
		return result, fmt.Errorf("%w: protocol hash differs from report specification", ErrInvalidCampaignExport)
	}
	evidence, err := readBoundedFile(spec.EvidencePath, maxCampaignSpecBytes)
	if err != nil {
		return result, fmt.Errorf("read export evidence manifest: %w", err)
	}
	report, err := BuildCampaignReport(ctx, dataset, spec.Report)
	if err != nil {
		return result, err
	}
	result.Summary = CampaignExportSummary{
		FormatVersion: report.FormatVersion, Campaign: report.Campaign, Status: report.Status,
		Phases: report.Phases, Offline: report.Offline, Budget: report.Budget, Recovery: report.Recovery,
		ReleaseApproved: false, Warnings: append([]string(nil), report.Warnings...),
	}
	result.Manifest = CampaignExportManifest{
		FormatVersion: campaignExportVersion, CampaignID: spec.Report.CampaignID,
		DatasetVersion: dataset.Version, DatasetManifestSHA256: dataset.ManifestSHA256,
		ProtocolSHA256: spec.Report.ProtocolSHA256, EvidenceManifestSHA256: digest(evidence),
		ExporterSourceCommit: spec.ExporterSourceCommit, BaselineFilesSHA256: cloneStringMap(spec.Report.BaselineFilesSHA256),
		Builds: map[string]*BuildProvenance{}, Plans: map[string]*Plan{}, GenerationConfigs: map[string]json.RawMessage{}, Artifacts: map[string]CampaignExportArtifact{},
		MissingData: []string{"actual_billed_cost", "response_completion_time"},
		PrivacyScan: CampaignExportPrivacyScan{Status: "pending", Method: "campaign-export-v1-deterministic-field-scan", Checks: []string{
			"API-key, private-key, Bearer and authorization-token patterns",
			"email addresses outside reserved .invalid fixtures",
			"phone-like values outside structured image_ref, selected_image_refs and reference fields",
			"prompt, inline image and private filesystem payload patterns",
		}, Limitations: []string{"Passing these deterministic checks is not a universal guarantee that arbitrary personal data is absent."}, RecordsScanned: map[string]int{"responses": 0, "semantic_criteria": 0, "errors": 0}, Findings: map[string]int{}},
		ReleaseApproved: false,
	}
	result.Manifest.Sources = append(result.Manifest.Sources,
		CampaignExportSource{ID: "dataset-manifest", Kind: "dataset_manifest", SHA256: dataset.ManifestSHA256},
		CampaignExportSource{ID: "protocol", Kind: "protocol", SHA256: digest(protocol)},
		CampaignExportSource{ID: "campaign-evidence", Kind: "evidence_manifest", SHA256: digest(evidence)},
	)
	if spec.RecoveryAddendumPath != "" {
		addendum, readErr := readBoundedFile(spec.RecoveryAddendumPath, maxCampaignSpecBytes)
		if readErr != nil {
			return result, fmt.Errorf("read recovery addendum: %w", readErr)
		}
		var identity struct {
			CampaignID string `json:"campaign_id"`
		}
		if json.Unmarshal(addendum, &identity) != nil || identity.CampaignID != spec.Report.CampaignID {
			return result, fmt.Errorf("%w: recovery addendum campaign mismatch", ErrInvalidCampaignExport)
		}
		result.Manifest.RecoveryAddendumSHA256 = digest(addendum)
		result.Manifest.Sources = append(result.Manifest.Sources, CampaignExportSource{ID: "recovery-addendum", Kind: "recovery_addendum", SHA256: digest(addendum)})
	}

	commits := map[string]string{"original_execution": spec.Report.SourceCommit, "exporter": spec.ExporterSourceCommit}
	caseRows := make(map[string]CampaignCaseTrial, len(report.Cases))
	for _, row := range report.Cases {
		caseRows[campaignExportSlotID(spec.Report.CampaignID, row.Phase, row.RequestedModel, row.CaseID, row.Trial)] = row
	}
	modelResolutions := map[string]*CampaignExportModelResolution{}
	allowedReferences, err := campaignExportKnownReferences(dataset)
	if err != nil {
		return result, err
	}
	recoveryObserved := false
	for _, phase := range spec.Report.Phases {
		for _, runSpec := range phase.Runs {
			if err = ctx.Err(); err != nil {
				return result, err
			}
			if err = appendCampaignExportRun(dataset, spec, phase, runSpec, caseRows, allowedReferences, &result, modelResolutions, commits); err != nil {
				return result, err
			}
			if runSpec.RecoveryEvidence != "" {
				recoveryObserved = true
			}
		}
	}
	if recoveryObserved && spec.RecoveryAddendumPath == "" {
		return result, fmt.Errorf("%w: recovery addendum is required for recovered evidence", ErrInvalidCampaignExport)
	}
	result.Manifest.Counts.PlannedSlots = report.Status.ExpectedSlots
	result.Manifest.Counts.ProviderAttempts = len(result.Attempts)
	result.Manifest.Counts.Responses = len(result.Responses)
	result.Manifest.Counts.Results = len(result.Results)
	result.Manifest.Counts.ReviewRows = len(result.Reviews)
	if len(result.Results) != result.Manifest.Counts.PlannedSlots {
		return result, fmt.Errorf("%w: every planned slot must retain one result", ErrInvalidCampaignExport)
	}
	for _, response := range result.Responses {
		if response.FormatStatus == "malformed" {
			result.Manifest.Counts.MalformedResponses++
		}
	}
	for _, review := range result.Reviews {
		if !review.EffectiveForResult {
			continue
		}
		result.Manifest.Counts.EffectiveReviews++
		switch {
		case review.Result == "unassessed":
			result.Manifest.Counts.UnassessedReviews++
		case review.ReviewerKind == "human":
			result.Manifest.Counts.HumanAssessments++
		case review.ReviewerKind == "agent":
			result.Manifest.Counts.AgentAssessments++
		}
	}
	result.Manifest.PrivacyScan.Status = "passed"
	for role, commit := range commits {
		result.Manifest.SourceCommits = append(result.Manifest.SourceCommits, CampaignExportSourceCommit{Role: role, Commit: commit})
	}
	slices.SortFunc(result.Manifest.SourceCommits, func(a, b CampaignExportSourceCommit) int { return strings.Compare(a.Role, b.Role) })
	for _, model := range modelResolutions {
		result.Manifest.Models = append(result.Manifest.Models, *model)
	}
	slices.SortFunc(result.Manifest.Models, func(a, b CampaignExportModelResolution) int {
		if a.Phase != b.Phase {
			return strings.Compare(a.Phase, b.Phase)
		}
		return strings.Compare(a.RequestedModel, b.RequestedModel)
	})
	slices.SortFunc(result.Manifest.Sources, func(a, b CampaignExportSource) int { return strings.Compare(a.ID, b.ID) })
	slices.Sort(result.Manifest.MissingData)
	return result, nil
}

func appendCampaignExportRun(dataset *Dataset, spec CampaignExportSpec, phase CampaignPhaseSpec, runSpec CampaignRunSpec, caseRows map[string]CampaignCaseTrial, allowedReferences map[string]bool, export *CampaignExport, models map[string]*CampaignExportModelResolution, commits map[string]string) error {
	record, attempts, err := ReadRun(runSpec.RunDirectory)
	if err != nil {
		return err
	}
	runData, err := readBoundedFile(runSpec.RunDirectory+"/run.json", maxCampaignSpecBytes)
	if err != nil {
		return err
	}
	attemptData, err := readBoundedFile(runSpec.RunDirectory+"/attempts.jsonl", 128*1024*1024)
	if err != nil {
		return err
	}
	runSourceID := campaignExportSourceID("run", phase.Name, runSpec.RequestedModel)
	attemptSourceID := campaignExportSourceID("attempts", phase.Name, runSpec.RequestedModel)
	export.Manifest.Sources = append(export.Manifest.Sources,
		CampaignExportSource{ID: runSourceID, Kind: "run_record", Phase: phase.Name, Model: runSpec.RequestedModel, SHA256: digest(runData), Records: 1},
		CampaignExportSource{ID: attemptSourceID, Kind: "attempt_journal", Phase: phase.Name, Model: runSpec.RequestedModel, SHA256: digest(attemptData), Records: len(attempts)},
	)
	if record.Build != nil {
		raw, marshalErr := json.Marshal(record.Build)
		if marshalErr != nil {
			return marshalErr
		}
		export.Manifest.Builds[digest(raw)] = record.Build
	}
	planRaw, marshalErr := json.Marshal(record.Plan)
	if marshalErr != nil {
		return marshalErr
	}
	export.Manifest.Plans[digest(planRaw)] = record.Plan
	runReferences := make(map[string]bool, len(allowedReferences))
	for reference, known := range allowedReferences {
		runReferences[reference] = known
	}
	for _, planned := range record.Plan.Cases {
		if planned.Variant == nil {
			continue
		}
		for _, base := range dataset.RK {
			if base.ID == planned.Variant.BaseCaseID {
				transformed, transformErr := TransformRanking(base.Input, planned.Variant.Transformation, planned.Variant.Seed)
				if transformErr != nil {
					return transformErr
				}
				for _, candidate := range transformed.Input.Candidates {
					runReferences[candidate.Reference] = true
				}
			}
		}
	}
	commits["original_execution"] = record.Commit
	modelKey := phase.Name + "/" + runSpec.RequestedModel
	resolution := &CampaignExportModelResolution{Phase: phase.Name, RequestedModel: runSpec.RequestedModel, ResolvedModelOccurrences: map[string]int{}}
	models[modelKey] = resolution

	baseAttemptIDs := map[string]string{}
	allAttemptIDs := map[string]string{}
	for index, attempt := range attempts {
		if attempt.RequestCount <= 0 {
			continue
		}
		id := campaignExportAttemptID(spec.Report.CampaignID, phase.Name, runSpec.RequestedModel, attempt.CaseID, attempt.Trial, "original", attempt.Retry)
		sequence := index + 1
		row := campaignExportAttemptRow(id, attemptSourceID, spec.Report.CampaignID, phase.Name, runSpec.RequestedModel, "original", attempt, &sequence, nil, "")
		if err = campaignExportRecordConfig(export, attempt.GenerationConfig); err != nil {
			return err
		}
		if row.SanitizedError != nil {
			if err = validateCampaignExportSensitiveText(*row.SanitizedError, false); err != nil {
				return fmt.Errorf("%w: unsafe sanitized attempt error", err)
			}
			export.Manifest.PrivacyScan.RecordsScanned["errors"]++
		}
		export.Attempts = append(export.Attempts, row)
		if err = appendCampaignPhysicalResponse(export, row, attempt, runReferences); err != nil {
			return err
		}
		baseAttemptIDs[attemptKey(attempt.CaseID, attempt.Trial)] = id
		allAttemptIDs[semanticAttemptKey(attempt.CaseID, attempt.Trial, attempt.Retry)] = id
		campaignExportAddResolution(resolution, row.ResolvedModel)
	}

	overlay, err := loadCampaignRecoveryOverlay(dataset, spec.Report, runSpec)
	if err != nil {
		return err
	}
	recoveryAttemptIDs := map[string]string{}
	if runSpec.RecoveryEvidence != "" {
		recoveryData, readErr := readBoundedFile(runSpec.RecoveryEvidence, 64*1024*1024)
		if readErr != nil {
			return readErr
		}
		recoverySourceID := campaignExportSourceID("recovery", phase.Name, runSpec.RequestedModel)
		export.Manifest.Sources = append(export.Manifest.Sources, CampaignExportSource{ID: recoverySourceID, Kind: "recovery_evidence", Phase: phase.Name, Model: runSpec.RequestedModel, SHA256: digest(recoveryData), Records: len(overlay.evidence.Attempts)})
		if commit := strings.TrimSpace(overlay.evidence.Manifest.RecoverySourceCommit); commit != "" {
			if observed := commits["recovery_execution"]; observed != "" && observed != commit {
				return fmt.Errorf("%w: recovery source commits differ", ErrInvalidCampaignExport)
			}
			commits["recovery_execution"] = commit
		}
		for _, item := range overlay.evidence.Attempts {
			number := item.RecoveryNumber
			id := campaignExportAttemptID(spec.Report.CampaignID, phase.Name, runSpec.RequestedModel, item.Attempt.CaseID, item.Attempt.Trial, "recovery", number)
			row := campaignExportAttemptRow(id, recoverySourceID, spec.Report.CampaignID, phase.Name, runSpec.RequestedModel, "recovery", item.Attempt, nil, &number, item.BaseAttemptSHA256)
			if err = campaignExportRecordConfig(export, item.Attempt.GenerationConfig); err != nil {
				return err
			}
			if row.SanitizedError != nil {
				if err = validateCampaignExportSensitiveText(*row.SanitizedError, false); err != nil {
					return fmt.Errorf("%w: unsafe sanitized recovery error", err)
				}
				export.Manifest.PrivacyScan.RecordsScanned["errors"]++
			}
			export.Attempts = append(export.Attempts, row)
			if err = appendCampaignPhysicalResponse(export, row, item.Attempt, runReferences); err != nil {
				return err
			}
			recoveryAttemptIDs[semanticAttemptKey(item.Attempt.CaseID, item.Attempt.Trial, item.Attempt.Retry)] = id
			allAttemptIDs[semanticAttemptKey(item.Attempt.CaseID, item.Attempt.Trial, item.Attempt.Retry)] = id
			campaignExportAddResolution(resolution, row.ResolvedModel)
		}
	}

	terminals, err := terminalAttempts(attempts)
	if err != nil {
		return err
	}
	effectiveBySlot := map[string]Attempt{}
	for key, original := range terminals {
		effectiveBySlot[key] = original
		if recovered, ok := overlay.slots[key]; ok {
			effectiveBySlot[key] = recovered.Effective
		}
	}
	keys := make([]string, 0, len(effectiveBySlot))
	for key := range effectiveBySlot {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		original := terminals[key]
		effective := effectiveBySlot[key]
		slotID := campaignExportSlotID(spec.Report.CampaignID, phase.Name, runSpec.RequestedModel, effective.CaseID, effective.Trial)
		caseRow, ok := caseRows[slotID]
		if !ok {
			return fmt.Errorf("%w: missing aggregate result for %s", ErrInvalidCampaignExport, slotID)
		}
		originalID := baseAttemptIDs[key]
		effectiveID := originalID
		if effective.Retry > original.Retry {
			effectiveID = recoveryAttemptIDs[semanticAttemptKey(effective.CaseID, effective.Trial, effective.Retry)]
		}
		responseID := ""
		outputSHA256 := ""
		var parsed json.RawMessage
		if effective.RawOutput != "" {
			if effectiveID == "" {
				return fmt.Errorf("%w: effective attempt identity missing for %s", ErrInvalidCampaignExport, slotID)
			}
			responseID = effectiveID + ":response"
			outputSHA256 = digest([]byte(effective.RawOutput))
			if effective.Status == "executed" && json.Valid([]byte(effective.RawOutput)) {
				parsed = json.RawMessage(effective.RawOutput)
			}
		}
		expected, observed, observationErr := campaignExportPlannedExpectedObserved(dataset, record.Plan, effective.CaseID, parsed)
		var observedMissing *string
		if observationErr != nil {
			if errors.Is(observationErr, ErrInvalidCampaignExport) {
				return observationErr
			}
			missing := "response_not_parseable"
			if effective.RawOutput == "" {
				missing = "response_not_available"
			}
			observedMissing = &missing
			observed = nil
		}
		export.Results = append(export.Results, CampaignExportResult{
			OutputSHA256: outputSHA256, SlotID: slotID, ResponseID: responseID, Phase: phase.Name, RequestedModel: runSpec.RequestedModel,
			CaseID: caseRow.CaseID, BaseCaseID: caseRow.BaseCaseID, FamilyID: caseRow.FamilyID, Split: caseRow.Split, Task: caseRow.Task, Trial: caseRow.Trial,
			OriginalStatus: caseRow.OriginalExecutionStatus, Recovered: caseRow.Recovered,
			RecoveryAttempts: caseRow.RecoveryAttempts, RecoveryReason: caseRow.RecoveryReason,
			Expected: expected, Observed: observed, ObservedMissing: observedMissing,
			DeterministicStatus: caseRow.DeterministicStatus, OverallStatus: caseRow.OverallStatus,
			SemanticStatus: caseRow.SemanticStatus, FailureCodes: append([]string(nil), caseRow.FailureCodes...),
			Metrics: caseRow.Metrics, SemanticCounts: caseRow.SemanticCounts,
			ReviewerKinds: append([]string(nil), caseRow.ReviewerKinds...), ReleaseApproved: false,
		})
	}

	if err = appendCampaignExportReviews(dataset, runSpec, phase.Name, spec.Report.CampaignID, effectiveBySlot, allAttemptIDs, export); err != nil {
		return err
	}
	for _, attempt := range effectiveBySlot {
		evaluation, evaluationErr := evaluatePlannedAttempt(dataset, record.Plan, attempt)
		if evaluationErr != nil {
			return evaluationErr
		}
		slotID := campaignExportSlotID(spec.Report.CampaignID, phase.Name, runSpec.RequestedModel, attempt.CaseID, attempt.Trial)
		attemptID := allAttemptIDs[semanticAttemptKey(attempt.CaseID, attempt.Trial, attempt.Retry)]
		for _, check := range evaluation.SemanticChecks {
			exists := false
			for _, review := range export.Reviews {
				if review.SlotID == slotID && review.EffectiveForResult && review.CriterionID == check.ID {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			export.Reviews = append(export.Reviews, CampaignExportReview{ReviewID: slotID + ":criterion:" + check.ID, SlotID: slotID, AttemptID: attemptID, Phase: phase.Name, RequestedModel: runSpec.RequestedModel, CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, EffectiveForResult: true, OutputSHA256: digest([]byte(attempt.RawOutput)), CriterionID: check.ID, CriterionSHA256: semanticCriterionHash(check), Criterion: check.Criterion, Severity: check.Severity, Result: "unassessed"})
		}
	}
	for i := range export.Results {
		row := &export.Results[i]
		if row.Phase != phase.Name || row.RequestedModel != runSpec.RequestedModel {
			continue
		}
		row.SemanticCriteria = []CampaignExportCriterion{}
		for _, criterion := range export.Reviews {
			if criterion.SlotID == row.SlotID && criterion.EffectiveForResult {
				row.SemanticCriteria = append(row.SemanticCriteria, CampaignExportCriterion{CriterionID: criterion.CriterionID, CriterionSHA256: criterion.CriterionSHA256, OutputSHA256: criterion.OutputSHA256, Severity: criterion.Severity, Result: criterion.Result})
			}
		}
	}
	for _, row := range export.Results {
		if row.Phase == phase.Name && row.RequestedModel == runSpec.RequestedModel {
			export.Manifest.PrivacyScan.RecordsScanned["semantic_criteria"] += len(row.SemanticCriteria)
		}
	}
	return nil
}

func appendCampaignExportReviews(dataset *Dataset, runSpec CampaignRunSpec, phase, campaignID string, effective map[string]Attempt, attemptIDs map[string]string, export *CampaignExport) error {
	appendReviews := func(document []SemanticReview, source, formatVersion string) {
		for _, review := range document {
			slotID := campaignExportSlotID(campaignID, phase, runSpec.RequestedModel, review.CaseID, review.Trial)
			attemptID := attemptIDs[semanticAttemptKey(review.CaseID, review.Trial, review.Retry)]
			effectiveAttempt := effective[attemptKey(review.CaseID, review.Trial)]
			effectiveReview := effectiveAttempt.Retry == review.Retry && digest([]byte(effectiveAttempt.RawOutput)) == review.OutputSHA256
			var responseID *string
			if effectiveReview && effectiveAttempt.RawOutput != "" {
				value := attemptID + ":response"
				responseID = &value
			}
			export.Reviews = append(export.Reviews, CampaignExportReview{
				ReviewID: attemptID + ":criterion:" + review.CriterionID, SlotID: slotID, AttemptID: attemptID,
				ResponseID: responseID, Phase: phase, RequestedModel: runSpec.RequestedModel,
				CaseID: review.CaseID, Trial: review.Trial, Retry: review.Retry, Source: source,
				ReviewFormatVersion: formatVersion,
				EffectiveForResult:  effectiveReview, OutputSHA256: review.OutputSHA256,
				CriterionID: review.CriterionID, CriterionSHA256: review.CriterionSHA256,
				Criterion: review.Criterion, Severity: review.Severity, Result: review.Result,
				Evidence: review.Evidence, EvidenceQuote: review.EvidenceQuote,
				EvidenceLocation: review.EvidenceLocation, Reason: review.Reason,
				Reviewer: review.Reviewer, ReviewerKind: review.ReviewerKind,
				ReviewedOn: review.ReviewedOn,
			})
		}
	}
	if runSpec.SemanticReviews != "" {
		_, err := readBoundedFile(runSpec.SemanticReviews, maxSemanticReviewBytes)
		if err != nil {
			return err
		}
		document, err := ReadSemanticReviews(runSpec.SemanticReviews)
		if err != nil {
			return err
		}
		if _, err = ApplySemanticReviews(dataset, runSpec.RunDirectory, document); err != nil {
			return err
		}

		appendReviews(document.Reviews, "original", document.FormatVersion)
	}
	if runSpec.RecoverySemanticReviews != "" {
		data, err := readBoundedFile(runSpec.RecoverySemanticReviews, maxSemanticReviewBytes)
		if err != nil {
			return err
		}
		var document RecoverySemanticReviewDocument
		if err = decodeCampaignJSON(data, &document); err != nil {
			return err
		}
		evidence, err := ReadCampaignRecovery(runSpec.RecoveryEvidence)
		if err != nil {
			return err
		}
		if _, err = ApplyRecoverySemanticReviews(dataset, runSpec.RunDirectory, evidence, document); err != nil {
			return err
		}

		appendReviews(document.Reviews, "recovery", document.FormatVersion)
	}
	return nil
}

func campaignExportAttemptRow(id, sourceID, campaignID, phase, model, source string, attempt Attempt, sequence, recoveryNumber *int, baseHash string) CampaignExportAttempt {
	latency := attempt.LatencyMillis
	row := CampaignExportAttempt{
		AttemptID: id, SlotID: campaignExportSlotID(campaignID, phase, model, attempt.CaseID, attempt.Trial),
		Phase: phase, RequestedModel: model, CaseID: attempt.CaseID, Trial: attempt.Trial,
		Source: source, SourceArtifactID: sourceID, JournalSequence: sequence, Retry: attempt.Retry,
		RecoveryNumber: recoveryNumber, BaseAttemptSHA256: baseHash, Status: attempt.Status,
		LatencyMillis: &latency, ProviderRequestCount: attempt.RequestCount,
		Provider: attempt.Provider, HTTPStatus: attempt.HTTPStatus, ProviderHTTPRequestID: attempt.RequestID, DurationScope: "attempt_including_mapping_generation_parsing", InputSHA256: attempt.InputSHA256, PromptSHA256: attempt.PromptSHA256,
		Usage: campaignExportAttemptUsage(attempt), MissingData: []string{"completion_time"},
	}
	if attempt.RequestID == nil {
		row.MissingData = append(row.MissingData, "provider_http_request_id")
	}
	if attempt.HTTPStatus == nil {
		row.MissingData = append(row.MissingData, "provider_http_status")
	}
	if row.Provider == "" {
		row.MissingData = append(row.MissingData, "provider")
	}
	if !attempt.StartedOn.IsZero() {
		started := attempt.StartedOn
		row.StartedOn = &started
	} else {
		row.LatencyMillis = nil
		row.MissingData = append(row.MissingData, "started_on", "latency_ms")
	}
	if len(attempt.GenerationConfig) > 0 {
		row.GenerationConfigSHA256 = digest(attempt.GenerationConfig)
	}
	if attempt.RawOutput != "" {
		row.RawOutputSHA256 = digest([]byte(attempt.RawOutput))
	}
	if attempt.Error != "" {
		value := sanitizeCampaignExportError(attempt.Error)
		row.SanitizedError = &value
		kind := campaignExportErrorClass(attempt)
		row.ErrorClass = &kind
	}
	var provider struct {
		ModelVersion string `json:"modelVersion"`
		ResponseID   string `json:"responseId"`
		Candidates   []struct {
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
	}
	providerErr := json.Unmarshal(attempt.ProviderResponse, &provider)
	row.FinishReasons = []string{}
	if providerErr == nil {
		if provider.ResponseID != "" {
			row.ProviderResponseID = &provider.ResponseID
		}
		if provider.PromptFeedback.BlockReason != "" {
			row.BlockReason = &provider.PromptFeedback.BlockReason
		}
		for _, candidate := range provider.Candidates {
			if candidate.FinishReason != "" {
				row.FinishReasons = append(row.FinishReasons, candidate.FinishReason)
			}
		}
	}
	if providerErr == nil && strings.TrimSpace(provider.ModelVersion) != "" {
		row.ResolvedModel = &provider.ModelVersion
	} else {
		row.MissingData = append(row.MissingData, "resolved_model")
	}
	slices.Sort(row.MissingData)
	return row
}

func campaignExportUsage(raw json.RawMessage) CampaignExportProviderUsage {
	result := CampaignExportProviderUsage{InputTokenDetails: []CampaignExportInputTokenDetail{}, Missing: map[string]string{}}
	var provider struct {
		Usage *struct {
			Input    *int64 `json:"promptTokenCount"`
			Output   *int64 `json:"candidatesTokenCount"`
			Thinking *int64 `json:"thoughtsTokenCount"`
			Cached   *int64 `json:"cachedContentTokenCount"`
			Total    *int64 `json:"totalTokenCount"`
			Details  []struct {
				Modality string `json:"modality"`
				Count    *int64 `json:"tokenCount"`
			} `json:"promptTokensDetails"`
		} `json:"usageMetadata"`
	}
	if len(raw) > 0 && json.Unmarshal(raw, &provider) == nil && provider.Usage != nil {
		result.InputTokens = provider.Usage.Input
		result.OutputTokens = provider.Usage.Output
		result.ThinkingTokens = provider.Usage.Thinking
		result.TotalTokens = provider.Usage.Total
		result.CachedInputTokens = provider.Usage.Cached
		for _, detail := range provider.Usage.Details {
			result.InputTokenDetails = append(result.InputTokenDetails, CampaignExportInputTokenDetail{Modality: detail.Modality, TokenCount: detail.Count})
		}
	}
	for name, value := range map[string]*int64{"input_tokens": result.InputTokens, "output_tokens": result.OutputTokens, "thinking_tokens": result.ThinkingTokens, "total_tokens": result.TotalTokens, "cached_input_tokens": result.CachedInputTokens} {
		if value == nil {
			result.Missing[name] = "not_reported_by_provider"
		}
	}
	if len(result.InputTokenDetails) == 0 {
		result.Missing["input_token_details"] = "not_reported_by_provider"
	}
	return result
}

func campaignExportExpectedObserved(dataset *Dataset, caseID string, parsed json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	for _, item := range dataset.PD {
		if item.ID != caseID {
			continue
		}
		if len(parsed) == 0 {
			return append(json.RawMessage(nil), item.Expected...), nil, fmt.Errorf("parsed output unavailable")
		}
		var output map[string]json.RawMessage
		if err := json.Unmarshal(parsed, &output); err != nil {
			return append(json.RawMessage(nil), item.Expected...), nil, err
		}
		var assessment map[string]json.RawMessage
		if raw, exists := output["assessment"]; exists {
			if err := json.Unmarshal(raw, &assessment); err != nil {
				return append(json.RawMessage(nil), item.Expected...), nil, err
			}
		}
		// Copy only reported fields so absent, null, empty and populated
		// values remain distinct in the exported observation.
		observed := map[string]json.RawMessage{}
		if status, exists := output["status"]; exists {
			observed["status"] = status
		}
		for source, target := range map[string]string{
			"action": "action", "outcome": "outcome",
			"problem_category_name": "category", "selected_image_refs": "selected_image_refs",
		} {
			if value, exists := assessment[source]; exists {
				observed[target] = value
			}
		}
		data, err := json.Marshal(observed)
		return append(json.RawMessage(nil), item.Expected...), data, err
	}
	for _, item := range dataset.RK {
		if item.ID != caseID {
			continue
		}
		if len(parsed) == 0 {
			return append(json.RawMessage(nil), item.Expected...), nil, fmt.Errorf("parsed output unavailable")
		}
		var output struct {
			Recommendations []struct {
				Reference string `json:"reference"`
			} `json:"recommendations"`
		}
		if err := json.Unmarshal(parsed, &output); err != nil {
			return append(json.RawMessage(nil), item.Expected...), nil, err
		}
		refs := make([]string, len(output.Recommendations))
		for i := range output.Recommendations {
			refs[i] = output.Recommendations[i].Reference
		}
		data, err := json.Marshal(struct {
			OrderedReferences []string `json:"ordered_references"`
		}{refs})
		return append(json.RawMessage(nil), item.Expected...), data, err
	}
	for _, item := range dataset.SM {
		if item.ID != caseID {
			continue
		}
		if len(parsed) == 0 {
			return append(json.RawMessage(nil), item.Expected...), nil, fmt.Errorf("parsed output unavailable")
		}
		var output map[string]json.RawMessage
		if err := json.Unmarshal(parsed, &output); err != nil {
			return item.Expected, nil, err
		}
		observed, err := json.Marshal(map[string]json.RawMessage{"summary": output["summary"]})
		return append(json.RawMessage(nil), item.Expected...), observed, err
	}
	return nil, nil, fmt.Errorf("%w: unknown case %s", ErrInvalidCampaignExport, caseID)
}

func campaignExportSlotID(campaign, phase, model, caseID string, trial int) string {
	return fmt.Sprintf("%s:%s:%s:%s:trial-%d", campaign, phase, model, caseID, trial)
}

func campaignExportAttemptID(campaign, phase, model, caseID string, trial int, source string, recoveryNumber int) string {
	base := campaignExportSlotID(campaign, phase, model, caseID, trial)
	if source == "recovery" {
		return fmt.Sprintf("%s:recovery:%d", base, recoveryNumber)
	}
	return fmt.Sprintf("%s:original:retry-%d", base, recoveryNumber)
}

func campaignExportSourceID(kind, phase, model string) string {
	return kind + ":" + phase + ":" + model
}

func campaignExportAddResolution(model *CampaignExportModelResolution, resolved *string) {
	if resolved == nil {
		model.UnknownAttempts++
		return
	}
	model.ResolvedModelOccurrences[*resolved]++
}

func campaignExportErrorClass(attempt Attempt) string {
	message := strings.ToLower(attempt.Error)
	switch {
	case attempt.RawOutput != "":
		return "output_parse_error"
	case strings.Contains(message, "503") || strings.Contains(message, "unavailable"):
		return "provider_unavailable"
	case strings.Contains(message, "504") || strings.Contains(message, "deadline") || strings.Contains(message, "timeout"):
		return "timeout"
	default:
		return "execution_error"
	}
}

var campaignExportURL = regexp.MustCompile(`https?://[^\s\"]+`)
var campaignExportAPIKey = regexp.MustCompile(`AIza[0-9A-Za-z_-]{20,}`)
var campaignExportEmail = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
var campaignExportPhone = regexp.MustCompile(`\+?[0-9][0-9\s().\-]{7,}[0-9]`)
var campaignExportUUID = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
var campaignExportSecrets = map[string]*regexp.Regexp{
	"google_api_key":      campaignExportAPIKey,
	"aws_access_key":      regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`),
	"github_token":        regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
	"openai_token":        regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`),
	"bearer_token":        regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{12,}`),
	"authorization_value": regexp.MustCompile(`(?i)\b(?:authorization|x-goog-api-key|api[_-]?key)\s*[:=]\s*["']?[^\s,"']{8,}`),
	"private_key":         regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

func sanitizeCampaignExportError(value string) string {
	value = campaignExportURL.ReplaceAllString(value, "[redacted_url]")
	return campaignExportAPIKey.ReplaceAllString(value, "[redacted_api_key]")
}

func campaignExportRecordConfig(export *CampaignExport, config json.RawMessage) error {
	if len(config) == 0 {
		return nil
	}
	if !json.Valid(config) {
		return fmt.Errorf("%w: generation configuration is not valid JSON", ErrInvalidCampaignExport)
	}
	if err := validateCampaignExportText(config); err != nil {
		return fmt.Errorf("%w: unsafe generation configuration", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, config); err != nil {
		return fmt.Errorf("%w: compact generation configuration: %v", ErrInvalidCampaignExport, err)
	}
	export.Manifest.GenerationConfigs[digest(config)] = append(json.RawMessage(nil), compact.Bytes()...)
	return nil
}

// validateCampaignExportResponse scans every public wire token without map
// decoding, which would discard duplicate keys in parser-rejected evidence.
func validateCampaignExportResponse(raw string, allowedReferences map[string]bool) error {
	if !json.Valid([]byte(raw)) {
		return validateCampaignExportSensitiveText(raw, false)
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := scanCampaignResponseToken(decoder, "", allowedReferences); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing response tokens", ErrInvalidCampaignExport)
	}
	return nil
}

func scanCampaignResponseToken(decoder *json.Decoder, field string, allowedReferences map[string]bool) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: decode response token: %v", ErrInvalidCampaignExport, err)
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				if keyErr != nil {
					return keyErr
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("%w: non-string response key", ErrInvalidCampaignExport)
				}
				if err := validateCampaignExportSensitiveText(key, false); err != nil {
					return err
				}
				if err := scanCampaignResponseToken(decoder, key, allowedReferences); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := scanCampaignResponseToken(decoder, field, allowedReferences); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("%w: unexpected response delimiter", ErrInvalidCampaignExport)
		}
		_, err = decoder.Token()
		return err
	case string:
		referenceField := field == "image_ref" || field == "selected_image_refs" || field == "reference"
		return validateCampaignExportSensitiveText(value, referenceField && allowedReferences[value])
	case json.Number:
		return validateCampaignExportSensitiveText(value.String(), false)
	}
	return nil
}

func campaignExportKnownReferences(dataset *Dataset) (map[string]bool, error) {
	result := map[string]bool{}
	for _, item := range dataset.PD {
		for _, image := range item.Input.Images {
			result["image:"+image.FileID] = true
		}
		var expected pdExpected
		if err := json.Unmarshal(item.Expected, &expected); err != nil {
			return nil, fmt.Errorf("decode expected references %s: %w", item.ID, err)
		}
		for _, reference := range append(append([]string(nil), expected.RequiredImages...), expected.AllowedImages...) {
			result[reference] = true
		}
	}
	for _, item := range dataset.RK {
		for _, candidate := range item.Input.Candidates {
			result[candidate.Reference] = true
		}
	}
	return result, nil
}

func validateCampaignExportSensitiveText(value string, skipPhone bool) error {
	for name, pattern := range campaignExportSecrets {
		if pattern.MatchString(value) {
			return fmt.Errorf("%w: %s pattern detected", ErrInvalidCampaignExport, name)
		}
	}
	for _, match := range campaignExportEmail.FindAllString(value, -1) {
		if !strings.HasSuffix(strings.ToLower(match), ".invalid") {
			return fmt.Errorf("%w: email address detected", ErrInvalidCampaignExport)
		}
	}
	if skipPhone {
		return nil
	}
	withoutIDs := campaignExportUUID.ReplaceAllString(value, "")
	for _, match := range campaignExportPhone.FindAllString(withoutIDs, -1) {
		digits := 0
		for _, character := range match {
			if character >= '0' && character <= '9' {
				digits++
			}
		}
		if digits >= 9 && digits <= 15 {
			return fmt.Errorf("%w: phone-like value detected", ErrInvalidCampaignExport)
		}
	}
	return nil
}

func validateCampaignExportText(data []byte) error {
	text := string(data)
	patterns := map[string]*regexp.Regexp{
		"image payload": regexp.MustCompile(`iVBORw0KGgo|"inlineData"\s*:`),
		"private path":  regexp.MustCompile(`/home/[^/\s\"]+/`),
	}
	for name, pattern := range campaignExportSecrets {
		patterns[name] = pattern
	}
	for name, pattern := range patterns {
		if pattern.MatchString(text) {
			return fmt.Errorf("%w: %s pattern detected", ErrInvalidCampaignExport, name)
		}
	}
	return nil
}

// ValidateCampaignExportArtifact applies the publication safety checks used by
// the offline writer. It does not mutate or redact evidence.
func ValidateCampaignExportArtifact(data []byte) error {
	return validateCampaignExportText(data)
}

// appendCampaignPhysicalResponse retains every received text unchanged, even
// when the adapter rejects it or a later recovery replaces the logical result.
func appendCampaignPhysicalResponse(export *CampaignExport, row CampaignExportAttempt, attempt Attempt, allowedReferences map[string]bool) error {
	if attempt.RawOutput == "" {
		return nil
	}
	if err := validateCampaignExportResponse(attempt.RawOutput, allowedReferences); err != nil {
		return fmt.Errorf("%w: unsafe response for %s", err, row.AttemptID)
	}
	export.Manifest.PrivacyScan.RecordsScanned["responses"]++
	raw := attempt.RawOutput
	response := CampaignExportResponse{ResponseID: row.AttemptID + ":response", SlotID: row.SlotID, Phase: row.Phase, RequestedModel: row.RequestedModel, DatasetCaseID: row.CaseID, Trial: row.Trial, OriginalAttemptID: row.AttemptID, EffectiveAttemptID: row.AttemptID, Recovered: row.Source == "recovery", ExecutionStatus: attempt.Status, FormatStatus: "malformed", InputSHA256: row.InputSHA256, PromptSHA256: row.PromptSHA256, GenerationConfigSHA: row.GenerationConfigSHA256, OutputSHA256: digest([]byte(raw)), RawOutput: &raw}
	if attempt.Status == "executed" && json.Valid([]byte(raw)) {
		response.ParsedOutput = json.RawMessage(raw)
		response.FormatStatus = "parsed"
	}
	export.Responses = append(export.Responses, response)
	export.Attempts[len(export.Attempts)-1].EffectiveResponseID = &response.ResponseID
	return nil
}

func campaignExportAttemptUsage(attempt Attempt) CampaignExportProviderUsage {
	raw := attempt.ProviderResponse
	source := attempt.UsageSource
	if len(attempt.ProviderUsage) > 0 {
		raw = append(append([]byte(`{"usageMetadata":`), attempt.ProviderUsage...), '}')
		if source == "" {
			source = "http_usage_metadata"
		}
	} else if source == "" {
		source = "sdk_decoded_presence_unknown"
	}
	usage := campaignExportUsage(raw)
	usage.Source = source
	return usage
}

// campaignExportPlannedExpectedObserved keeps observations in the original wire
// reference space. Only the oracle is renamed for a generated bijection.
func campaignExportPlannedExpectedObserved(dataset *Dataset, plan *Plan, caseID string, parsed json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	for _, planned := range plan.Cases {
		if planned.CaseID != caseID {
			continue
		}
		if planned.Variant == nil {
			return campaignExportExpectedObserved(dataset, caseID, parsed)
		}
		variant := planned.Variant
		for _, base := range dataset.RK {
			if base.ID != variant.BaseCaseID {
				continue
			}
			transformed, err := TransformRanking(base.Input, variant.Transformation, variant.Seed)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: transform oracle: %v", ErrInvalidCampaignExport, err)
			}
			expected, observed, observationErr := campaignExportExpectedObserved(dataset, base.ID, parsed)
			if len(transformed.InverseReferences) == 0 {
				return expected, observed, observationErr
			}
			forward := make(map[string]string, len(transformed.InverseReferences))
			for renamed, original := range transformed.InverseReferences {
				forward[original] = renamed
			}
			expected, err = campaignExportRenameExpectedReferences(expected, forward)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: transform oracle references: %v", ErrInvalidCampaignExport, err)
			}
			return expected, observed, observationErr
		}
		return nil, nil, fmt.Errorf("%w: unknown variant parent %s", ErrInvalidCampaignExport, variant.BaseCaseID)
	}
	return nil, nil, fmt.Errorf("%w: unknown planned case %s", ErrInvalidCampaignExport, caseID)
}

func campaignExportRenameExpectedReferences(raw json.RawMessage, forward map[string]string) (json.RawMessage, error) {
	var expected map[string]json.RawMessage
	if err := json.Unmarshal(raw, &expected); err != nil {
		return nil, err
	}
	for _, field := range []string{"eligible_references", "relevance", "pairwise_constraints", "tie_groups"} {
		value, exists := expected[field]
		if !exists {
			continue
		}
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			return nil, err
		}
		var rename func(any) any
		rename = func(value any) any {
			switch typed := value.(type) {
			case string:
				if replacement, exists := forward[typed]; exists {
					return replacement
				}
				return typed
			case []any:
				for i := range typed {
					typed[i] = rename(typed[i])
				}
				return typed
			case map[string]any:
				replaced := make(map[string]any, len(typed))
				for key, child := range typed {
					if replacement, exists := forward[key]; exists {
						key = replacement
					}
					replaced[key] = rename(child)
				}
				return replaced
			default:
				return value
			}
		}
		encoded, err := json.Marshal(rename(decoded))
		if err != nil {
			return nil, err
		}
		expected[field] = encoded
	}
	return json.Marshal(expected)
}
