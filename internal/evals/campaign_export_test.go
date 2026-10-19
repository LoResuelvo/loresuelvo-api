package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"

	"github.com/stretchr/testify/require"
)

func TestBuildCampaignExportPreservesAuditableResponseAndAttemptData(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "executed")
	reportSpec := campaignReportSpec(t, dataset, runDirectory)
	protocolPath := filepath.Join(t.TempDir(), "protocol.json")
	protocol := []byte(`{"protocol":"fixture"}`)
	require.NoError(t, os.WriteFile(protocolPath, protocol, 0600))
	reportSpec.ProtocolSHA256 = digest(protocol)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"evidence":"fixture"}`), 0600))

	bundle, err := BuildCampaignExport(context.Background(), dataset, CampaignExportSpec{
		Report:               reportSpec,
		ProtocolPath:         protocolPath,
		EvidencePath:         evidencePath,
		ExporterSourceCommit: "exporter-commit",
	})
	require.NoError(t, err)
	require.Equal(t, "campaign-1", bundle.Manifest.CampaignID)
	require.Equal(t, "exporter-commit", bundle.Manifest.ExporterSourceCommit)
	require.Equal(t, 1, bundle.Manifest.Counts.PlannedSlots)
	require.Equal(t, 1, bundle.Manifest.Counts.ProviderAttempts)
	require.Equal(t, 1, bundle.Manifest.Counts.Responses)
	require.Equal(t, 1, bundle.Manifest.Counts.Results)
	require.Zero(t, bundle.Manifest.Counts.MalformedResponses)
	require.Len(t, bundle.Attempts, 1)
	require.Len(t, bundle.Responses, 1)
	require.Len(t, bundle.Results, 1)

	attempt := bundle.Attempts[0]
	require.Equal(t, "campaign-1:smoke:model-a:RK-test:trial-1:original:retry-0", attempt.AttemptID)
	require.Equal(t, "original", attempt.Source)
	require.Nil(t, attempt.ProviderHTTPRequestID)
	require.Equal(t, int64(12), *attempt.Usage.InputTokens)
	require.Equal(t, int64(3), *attempt.Usage.OutputTokens)
	require.Nil(t, attempt.Usage.ThinkingTokens)
	require.Equal(t, "not_reported_by_provider", attempt.Usage.Missing["thinking_tokens"])
	require.Equal(t, "resolved-model", *attempt.ResolvedModel)
	require.Contains(t, bundle.Manifest.GenerationConfigs, attempt.GenerationConfigSHA256)
	require.JSONEq(t, `{"responseMimeType":"application/json","maxOutputTokens":128}`, string(bundle.Manifest.GenerationConfigs[attempt.GenerationConfigSHA256]))

	response := bundle.Responses[0]
	require.Equal(t, attempt.AttemptID, response.EffectiveAttemptID)
	require.NotNil(t, response.RawOutput)
	require.JSONEq(t, string(rankingRaw(t, []string{"a", "b", "c"})), *response.RawOutput)
	require.NotEmpty(t, response.OutputSHA256)
	require.JSONEq(t, string(rankingRaw(t, []string{"a", "b", "c"})), string(response.ParsedOutput))
	require.Equal(t, "parsed", response.FormatStatus)

	result := bundle.Results[0]
	require.Equal(t, response.ResponseID, result.ResponseID)
	require.Equal(t, "ranking", result.Task)
	require.Equal(t, "RK-test", result.BaseCaseID)
	require.Equal(t, "executed", result.OriginalStatus)
	require.False(t, result.Recovered)
	require.NotEmpty(t, result.Expected)
	require.JSONEq(t, `{"ordered_references":["a","b","c"]}`, string(result.Observed))
	require.Equal(t, "passed", result.DeterministicStatus)
	require.False(t, result.ReleaseApproved)
	require.False(t, bundle.Summary.ReleaseApproved)
	require.NotContains(t, bundle.Manifest.MissingData, "human_review")
	require.Contains(t, bundle.Manifest.MissingData, "actual_billed_cost")
	require.NotEmpty(t, bundle.Manifest.Sources)
	for _, source := range bundle.Manifest.Sources {
		require.NotContains(t, source.ID, t.TempDir())
		require.NotEmpty(t, source.SHA256)
	}
}

func TestBuildCampaignExportKeepsMalformedPhysicalResponse(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "executed")
	record, attempts, err := ReadRun(runDirectory)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	malformed := attempts[0]
	malformed.Status = "execution_error"
	malformed.Error = "execution_error: parsing provider ranking response"
	malformed.RawOutput = `[{"reference":"a"}]`
	malformed.ParsedOutput = nil
	record.Status = "partial"
	runDirectory = persistReplayEvidence(t, record, malformed)
	reportSpec := campaignReportSpec(t, dataset, runDirectory)
	reportSpec.Budget.ObservedGenerationCalls = 1
	reportSpec.Budget.GuardRecordedUsageCalls = 1
	protocolPath := filepath.Join(t.TempDir(), "protocol.json")
	protocol := []byte(`{"protocol":"fixture"}`)
	require.NoError(t, os.WriteFile(protocolPath, protocol, 0600))
	reportSpec.ProtocolSHA256 = digest(protocol)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"evidence":"fixture"}`), 0600))

	bundle, err := BuildCampaignExport(context.Background(), dataset, CampaignExportSpec{
		Report: reportSpec, ProtocolPath: protocolPath, EvidencePath: evidencePath,
		ExporterSourceCommit: "exporter-commit",
	})
	require.NoError(t, err)
	require.Equal(t, 1, bundle.Manifest.Counts.MalformedResponses)
	require.Len(t, bundle.Responses, 1)
	require.Equal(t, "malformed", bundle.Responses[0].FormatStatus)
	require.Equal(t, malformed.RawOutput, *bundle.Responses[0].RawOutput)
	require.Nil(t, bundle.Responses[0].ParsedOutput)
	require.Equal(t, "response_not_parseable", *bundle.Results[0].ObservedMissing)
	require.Equal(t, "output_parse_error", *bundle.Attempts[0].ErrorClass)
}

func TestBuildCampaignExportPreservesMissingPhysicalResponse(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "not_executed")
	reportSpec := campaignReportSpec(t, dataset, runDirectory)
	protocolPath := filepath.Join(t.TempDir(), "protocol.json")
	protocol := []byte(`{"protocol":"fixture"}`)
	require.NoError(t, os.WriteFile(protocolPath, protocol, 0600))
	reportSpec.ProtocolSHA256 = digest(protocol)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"evidence":"fixture"}`), 0600))

	bundle, err := BuildCampaignExport(context.Background(), dataset, CampaignExportSpec{
		Report: reportSpec, ProtocolPath: protocolPath, EvidencePath: evidencePath,
		ExporterSourceCommit: "exporter-commit",
	})
	require.NoError(t, err)
	require.Len(t, bundle.Results, 1)
	require.Empty(t, bundle.Responses)
	require.Equal(t, "response_not_available", *bundle.Results[0].ObservedMissing)
	require.Empty(t, bundle.Results[0].OutputSHA256)
}

func TestCampaignExportSanitizesErrorsAndRejectsSecrets(t *testing.T) {
	sanitized := sanitizeCampaignExportError(`Post "https://example.test/path?key=secret": context deadline exceeded`)
	require.NotContains(t, sanitized, "https://")
	require.NotContains(t, sanitized, "secret")
	require.Contains(t, sanitized, "[redacted_url]")
	require.NoError(t, validateCampaignExportText([]byte(`{"response":"ordinary text"}`)))
	require.Error(t, validateCampaignExportText([]byte(`{"response":"AIza012345678901234567890123456789"}`)))
	require.Error(t, validateCampaignExportText([]byte(`{"response":"iVBORw0KGgoAAA"}`)))
	references := map[string]bool{"candidate-123456789012": true}
	require.Error(t, validateCampaignExportResponse(`{"content":"contact test@example.com"}`, references))
	require.Error(t, validateCampaignExportResponse(`{"content":"llamame al +54 11 5555 1234"}`, references))
	require.Error(t, validateCampaignExportResponse(`{"content":"Authorization: Bearer abcdefghijklmnopqrstuvwxyz"}`, references))
	require.Error(t, validateCampaignExportResponse(`{"reference":"+54 11 5555 1234"}`, references))
	require.Error(t, validateCampaignExportResponse(`{"image_ref":"+54 11 5555 1234"}`, references))
	require.NoError(t, validateCampaignExportResponse(`{"reference":"candidate-123456789012"}`, references))
	require.NoError(t, validateCampaignExportResponse(`{"content":"fixture@example.invalid"}`, references))
	row := campaignExportAttemptRow("attempt", "source", "campaign", "phase", "model", "original", Attempt{CaseID: "case", Trial: 1, RequestCount: 1}, nil, nil, "")
	require.Nil(t, row.StartedOn)
	require.Contains(t, row.MissingData, "started_on")
}

func TestCampaignExportOriginalRetryIDsAreUnique(t *testing.T) {
	first := campaignExportAttemptID("campaign", "primary", "model", "case", 1, "original", 0)
	second := campaignExportAttemptID("campaign", "primary", "model", "case", 1, "original", 1)
	require.NotEqual(t, first, second)
	require.Equal(t, "campaign:primary:model:case:trial-1:original:retry-0", first)
	require.Equal(t, "campaign:primary:model:case:trial-1:original:retry-1", second)
}

func TestCampaignExportPredictionPreservesExpectedAndObservedDimensions(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	raw := rankingRaw(t, []string{"c", "a", "b"})
	expected, observed, err := campaignExportExpectedObserved(dataset, "RK-test", raw)
	require.NoError(t, err)
	require.NotEmpty(t, expected)
	var decoded map[string][]string
	require.NoError(t, json.Unmarshal(observed, &decoded))
	require.Equal(t, []string{"c", "a", "b"}, decoded["ordered_references"])
}

func TestBuildCampaignExportProjectsPDWireResponseInsteadOfDomainJSON(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "executed")
	record, attempts, err := ReadRun(runDirectory)
	require.NoError(t, err)
	dataset.RK = nil
	dataset.PD = []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-test", Task: "prediagnosis", FamilyID: "synthetic-plumbing", Split: "development"},
		Input:    PDInput{AvailableCategories: []string{"Plomería"}, RecentMessages: []MessageInput{{SenderRole: "consumer", Images: []HistoricalImage{{FileID: "one"}, {FileID: "two"}}}}},
		Expected: json.RawMessage(`{"statuses":["answered"],"actions":["replace"],"outcomes":["professional_required"],"category_names":["Plomería"]}`)}}
	dataset.schemaFiles = evaluationSchemaFixture(t, "prediagnosis-output")
	dataset.Suites["smoke"] = []string{"PD-test"}
	record.Plan.Cases[0].CaseID = "PD-test"
	attempt := attempts[0]
	attempt.CaseID = "PD-test"
	attempt.RawOutput = `{"status":"answered","title":"Pérdida","content":"Solicitá revisión.","image_descriptions":[],"assessment":{"action":"replace","outcome":"professional_required","problem_title":"Pérdida","problem_description":"Goteo bajo pileta.","problem_category_name":"Plomería","selected_image_refs":["image:one","image:two"]}}`
	attempt.ParsedOutput, err = json.Marshal(conversation.ChatbotResponse{
		Status: conversation.ChatbotResponseAnswered, Title: "Pérdida", Content: "Solicitá revisión.",
		Assessment: conversation.ChatbotAssessmentResponse{Action: conversation.ChatbotAssessmentReplace,
			Outcome: "professional_required", ProblemTitle: "Pérdida", ProblemDescription: "Goteo bajo pileta.",
			ProblemCategoryName: "Plomería", SelectedImageRefs: []string{"image:one", "image:two"}},
	})
	require.NoError(t, err)
	runDirectory = persistReplayEvidence(t, record, attempt)
	reportSpec := campaignReportSpec(t, dataset, runDirectory)
	protocolPath := filepath.Join(t.TempDir(), "protocol.json")
	protocol := []byte(`{"protocol":"fixture"}`)
	require.NoError(t, os.WriteFile(protocolPath, protocol, 0600))
	reportSpec.ProtocolSHA256 = digest(protocol)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"evidence":"fixture"}`), 0600))
	bundle, err := BuildCampaignExport(context.Background(), dataset, CampaignExportSpec{
		Report: reportSpec, ProtocolPath: protocolPath, EvidencePath: evidencePath, ExporterSourceCommit: "exporter-commit",
	})
	require.NoError(t, err)
	require.Len(t, bundle.Responses, 1)
	require.Equal(t, attempt.RawOutput, *bundle.Responses[0].RawOutput)
	require.Equal(t, digest([]byte(attempt.RawOutput)), bundle.Responses[0].OutputSHA256)
	require.JSONEq(t, attempt.RawOutput, string(bundle.Responses[0].ParsedOutput))
	require.JSONEq(t, `{"status":"answered","action":"replace","outcome":"professional_required","category":"Plomería","selected_image_refs":["image:one","image:two"]}`, string(bundle.Results[0].Observed))
}

func TestCampaignExportPDObservationPreservesWireFieldPresence(t *testing.T) {
	dataset := &Dataset{PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-test"}, Expected: json.RawMessage(`{}`)}}}
	for _, tc := range []struct{ name, raw, observed string }{
		{"empty", `{"status":"answered","assessment":{"action":"unchanged","outcome":"","problem_category_name":"","selected_image_refs":[]}}`, `{"status":"answered","action":"unchanged","outcome":"","category":"","selected_image_refs":[]}`},
		{"null", `{"status":null,"assessment":{"action":null,"outcome":null,"problem_category_name":null,"selected_image_refs":null}}`, `{"status":null,"action":null,"outcome":null,"category":null,"selected_image_refs":null}`},
		{"missing", `{"assessment":{}}`, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, observed, err := campaignExportExpectedObserved(dataset, "PD-test", json.RawMessage(tc.raw))
			require.NoError(t, err)
			require.JSONEq(t, tc.observed, string(observed))
		})
	}
}

func TestCampaignExportOmitsPrivateAssessmentDetails(t *testing.T) {
	review := CampaignExportReview{CriterionID: "c", CriterionSHA256: "hash", Result: "pass", Reviewer: "private-name", ReviewerKind: "agent", Reason: "private reasoning", Evidence: "private evidence"}
	raw, err := json.Marshal(review)
	require.NoError(t, err)
	for _, private := range []string{"private-name", "agent", "private reasoning", "private evidence", "reviewer"} {
		require.NotContains(t, string(raw), private)
	}
	result, err := json.Marshal(CampaignExportResult{ReviewerKinds: []string{"agent"}})
	require.NoError(t, err)
	require.NotContains(t, string(result), "reviewer")
}

func campaignExportFixtureSpec(t *testing.T, dataset *Dataset, directory string) CampaignExportSpec {
	t.Helper()
	report := campaignReportSpec(t, dataset, directory)
	protocol := []byte(`{"protocol":"fixture"}`)
	protocolPath := filepath.Join(t.TempDir(), "protocol.json")
	require.NoError(t, os.WriteFile(protocolPath, protocol, 0600))
	report.ProtocolSHA256 = digest(protocol)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"evidence":"fixture"}`), 0600))
	return CampaignExportSpec{Report: report, ProtocolPath: protocolPath, EvidencePath: evidencePath, ExporterSourceCommit: "exporter-commit"}
}

func TestBuildCampaignExportRetainsNonterminalRejectedResponse(t *testing.T) {
	dataset, directory := campaignReportFixture(t, "executed")
	record, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	record.Plan.MaxRetries = 1
	record.Plan.MaximumRequests = 2
	record.Plan.RequestLimit = 2
	rejected := attempts[0]
	rejected.Status = "execution_error"
	rejected.Error = "execution_error: invalid provider response"
	rejected.RawOutput = `{"rejected":true}`
	rejected.ParsedOutput = nil
	accepted := attempts[0]
	accepted.Retry = 1
	directory = persistReplayEvidence(t, record, rejected, accepted)
	spec := campaignExportFixtureSpec(t, dataset, directory)
	spec.Report.Budget.ObservedGenerationCalls = 2
	spec.Report.Budget.GuardRecordedUsageCalls = 2
	bundle, err := BuildCampaignExport(context.Background(), dataset, spec)
	require.NoError(t, err)
	require.Len(t, bundle.Responses, 2)
	require.Len(t, bundle.Results, 1)
	require.Len(t, bundle.Attempts, 2)
	require.Equal(t, rejected.RawOutput, *bundle.Responses[0].RawOutput)
	require.Nil(t, bundle.Responses[0].ParsedOutput)
	require.Equal(t, "execution_error", bundle.Responses[0].ExecutionStatus)
	require.Equal(t, accepted.RawOutput, *bundle.Responses[1].RawOutput)
	require.Equal(t, bundle.Responses[1].ResponseID, bundle.Results[0].ResponseID)
	require.Equal(t, digest([]byte(accepted.RawOutput)), bundle.Results[0].OutputSHA256)
	require.NotEqual(t, digest([]byte(rejected.RawOutput)), bundle.Results[0].OutputSHA256)
	require.Equal(t, 2, bundle.Summary.Phases[0].Models[0].Operations.Requests)
	require.Equal(t, int64(24), *bundle.Summary.Phases[0].Models[0].Operations.InputTokens.ObservedTotal)
	require.NotContains(t, bundle.Manifest.PrivacyScan.RecordsScanned, "review_evidence")
	require.Equal(t, 2, bundle.Manifest.PrivacyScan.RecordsScanned["semantic_criteria"])
	require.Len(t, bundle.Results[0].SemanticCriteria, 2)
	require.Equal(t, "unassessed", bundle.Results[0].SemanticCriteria[0].Result)
}

func TestBuildCampaignExportPublicAssessmentHasNoPrivateIdentityOrSource(t *testing.T) {
	dataset, directory := campaignReportFixture(t, "executed")
	reviews, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	reviews.Reviews[0].Result = "pass"
	reviews.Reviews[0].Reviewer = "persona@example.com"
	reviews.Reviews[0].ReviewerKind = "agent"
	reviews.Reviews[0].ReviewedOn = &now
	reviews.Reviews[0].EvidenceLocation = "absent"
	reviews.Reviews[0].Reason = "private-reasoning contacto +54 11 5555 1234"
	reviews.Reviews[0].Evidence = "private-evidence persona@example.com"
	path := filepath.Join(t.TempDir(), "private-reviews.json")
	require.NoError(t, WriteSemanticReviews(path, reviews))
	spec := campaignExportFixtureSpec(t, dataset, directory)
	spec.Report.Phases[0].Runs[0].SemanticReviews = path
	bundle, err := BuildCampaignExport(context.Background(), dataset, spec)
	require.NoError(t, err)
	require.Equal(t, "pass", bundle.Results[0].SemanticCriteria[0].Result)
	for _, value := range []any{bundle.Manifest, bundle.Results, bundle.Summary} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		for _, private := range []string{"persona@example.com", "private-reasoning", "private-evidence", "reviewer_kind", "agent_assessments", "human_review", "semantic_review_sha256", "private-reviews.json"} {
			require.NotContains(t, string(raw), private)
		}
	}
	for _, source := range bundle.Manifest.Sources {
		require.NotContains(t, source.Kind, "semantic_reviews")
	}
}

func TestBuildCampaignReportAndExportSummaryIsSemanticallyUnassessed(t *testing.T) {
	dataset := &Dataset{Version: "test-v1", ManifestSHA256: "manifest", SM: []SMCase{{CaseMetadata: CaseMetadata{ID: "SM-one", Task: "summarization", FamilyID: "summary", Split: "development"}, Expected: json.RawMessage(`{"facts_to_preserve":["fact"],"must_not_invent":["cause"],"semantic_assertions":["keep uncertainty"],"required_image_refs":[]}`)}}, Suites: map[string][]string{"smoke": {"SM-one"}}, schemaFiles: evaluationSchemaFixture(t, "summarization-output")}
	_, directory := campaignReportFixture(t, "executed")
	record, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	record.Plan.Cases[0].CaseID = "SM-one"
	attempt := attempts[0]
	attempt.CaseID = "SM-one"
	attempt.RawOutput = `{"summary":"Simulated summary."}`
	attempt.ParsedOutput = json.RawMessage(`"Simulated summary."`)
	directory = persistReplayEvidence(t, record, attempt)
	bundle, err := BuildCampaignExport(context.Background(), dataset, campaignExportFixtureSpec(t, dataset, directory))
	require.NoError(t, err)
	require.Equal(t, "summarization", bundle.Results[0].Task)
	require.Equal(t, "unassessed", bundle.Results[0].SemanticStatus)
	require.Len(t, bundle.Results[0].SemanticCriteria, 4)
	require.JSONEq(t, attempt.RawOutput, string(bundle.Responses[0].ParsedOutput))
	require.JSONEq(t, attempt.RawOutput, string(bundle.Results[0].Observed))
	task := bundle.Summary.Phases[0].Models[0].Tasks["summarization"]
	require.Equal(t, 1, task.Coverage.ExpectedSlots)
	require.Empty(t, task.TrialVariability)
	for _, criterion := range bundle.Results[0].SemanticCriteria {
		if criterion.CriterionID == "shared_readability" {
			require.Equal(t, "minor", criterion.Severity)
		} else {
			require.Empty(t, criterion.Severity)
		}
		require.NotEmpty(t, criterion.CriterionSHA256)
	}
}

func TestCampaignExportHTTPUsagePreservesReportedZeroAndIdentifiers(t *testing.T) {
	status := 200
	requestID := "request-id"
	attempt := Attempt{Provider: "gemini", RequestCount: 1, HTTPStatus: &status, RequestID: &requestID, ProviderUsage: json.RawMessage(`{"promptTokenCount":12,"candidatesTokenCount":0,"thoughtsTokenCount":0,"cachedContentTokenCount":0,"totalTokenCount":12}`), ProviderResponse: json.RawMessage(`{"responseId":"response-id","modelVersion":"resolved","candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":999}}`)}
	row := campaignExportAttemptRow("attempt", "source", "campaign", "phase", "model", "original", attempt, nil, nil, "")
	require.Equal(t, int64(12), *row.Usage.InputTokens)
	require.Equal(t, int64(0), *row.Usage.ThinkingTokens)
	require.Equal(t, int64(0), *row.Usage.OutputTokens)
	require.Equal(t, int64(0), *row.Usage.CachedInputTokens)
	require.Equal(t, "response-id", *row.ProviderResponseID)
	require.Equal(t, "request-id", *row.ProviderHTTPRequestID)
	require.Equal(t, 200, *row.HTTPStatus)
	require.Equal(t, []string{"STOP"}, row.FinishReasons)
	require.NotContains(t, row.Usage.Missing, "thinking_tokens")
	attempt.ProviderUsage = nil
	row = campaignExportAttemptRow("attempt", "source", "campaign", "phase", "model", "original", attempt, nil, nil, "")
	require.Nil(t, row.Usage.ThinkingTokens)
	require.Equal(t, "sdk_decoded_presence_unknown", row.Usage.Source)
}

func TestBuildCampaignExportVariantsKeepWireReferencesAndTransformExpected(t *testing.T) {
	for _, kind := range []string{OpaqueReferenceBijection, CandidatePermutation} {
		t.Run(kind, func(t *testing.T) {
			dataset, directory := campaignReportFixture(t, "executed")
			record, attempts, err := ReadRun(directory)
			require.NoError(t, err)
			dataset.RK[0].Input.Candidates = []CandidateInput{{Reference: "a"}, {Reference: "b"}, {Reference: "c"}}
			dataset.Metamorphic.Ranking = []RankingTransformationPlan{{BaseCaseID: "RK-test", FamilyID: dataset.RK[0].FamilyID, Split: "development", Transformations: []string{kind}, Seeds: []int64{7}, RepeatTrials: 1, AggregationUnit: "base_case_not_variant"}}
			plan, err := BuildPlan(dataset, PlanOptions{Suite: "smoke", Model: "model-a", Trials: 1, MaxRequests: 2, Metamorphic: true})
			require.NoError(t, err)
			record.Plan = plan
			variant := attempts[0]
			variant.CaseID = plan.Cases[1].CaseID
			transformed, err := TransformRanking(dataset.RK[0].Input, kind, 7)
			require.NoError(t, err)
			refs := []string{"a", "b", "c"}
			for i, old := range refs {
				for renamed, base := range transformed.InverseReferences {
					if old == base {
						refs[i] = renamed
					}
				}
			}
			variant.RawOutput = string(rankingRaw(t, refs))
			variant.ParsedOutput = json.RawMessage(variant.RawOutput)
			directory = persistReplayEvidence(t, record, attempts[0], variant)
			spec := campaignExportFixtureSpec(t, dataset, directory)
			spec.Report.Phases[0].Metamorphic = true
			spec.Report.Budget.ExpectedGenerationCalls = 2
			spec.Report.Budget.ObservedGenerationCalls = 2
			spec.Report.Budget.GuardRecordedUsageCalls = 2
			bundle, err := BuildCampaignExport(context.Background(), dataset, spec)
			require.NoError(t, err)
			var result *CampaignExportResult
			for i := range bundle.Results {
				if bundle.Results[i].CaseID == variant.CaseID {
					result = &bundle.Results[i]
				}
			}
			require.NotNil(t, result)
			require.Nil(t, result.ObservedMissing)
			require.NotEmpty(t, result.Expected)
			observed, err := json.Marshal(map[string]any{"ordered_references": refs})
			require.NoError(t, err)
			require.JSONEq(t, string(observed), string(result.Observed))
			var expected rkExpected
			require.NoError(t, json.Unmarshal(result.Expected, &expected))
			require.Equal(t, refs, expected.Eligible)
			require.Equal(t, 3, expected.Relevance[refs[0]])
			require.Equal(t, refs[0], expected.Pairs[0].Higher)
			require.Equal(t, refs[1], expected.Pairs[0].Lower)
		})
	}
}

func TestCampaignExportPrivacyChecksAllPublicRawTokens(t *testing.T) {
	for _, raw := range []string{`{"content":"contact test@example.com","content":"safe"}`, `{"test@example.com":"safe"}`, `{"phone":541155551234}`, `{"content":"safe","content":"contact test@example.com"}`} {
		require.Error(t, validateCampaignExportResponse(raw, nil), raw)
	}
	require.NoError(t, validateCampaignExportResponse(`{"content":"first","content":"second"}`, nil))
}

func TestBuildCampaignExportWhitespaceResponseRemainsPhysicalEvidence(t *testing.T) {
	dataset, directory := campaignReportFixture(t, "executed")
	record, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	attempt := attempts[0]
	attempt.Status = "execution_error"
	attempt.Error = "execution_error: parsing provider response"
	attempt.RawOutput = " \t"
	attempt.ParsedOutput = nil
	directory = persistReplayEvidence(t, record, attempt)
	bundle, err := BuildCampaignExport(context.Background(), dataset, campaignExportFixtureSpec(t, dataset, directory))
	require.NoError(t, err)
	require.Len(t, bundle.Responses, 1)
	require.Equal(t, " \t", *bundle.Responses[0].RawOutput)
	require.Equal(t, bundle.Responses[0].ResponseID, bundle.Results[0].ResponseID)
	require.Equal(t, digest([]byte(" \t")), bundle.Results[0].OutputSHA256)
	require.Equal(t, "malformed", bundle.Responses[0].FormatStatus)
	require.Equal(t, "failed", bundle.Results[0].OverallStatus)
	require.Equal(t, "unassessed", bundle.Results[0].SemanticStatus)
	require.Equal(t, "response_not_parseable", *bundle.Results[0].ObservedMissing)
	require.Equal(t, "output_parse_error", *bundle.Attempts[0].ErrorClass)
}
