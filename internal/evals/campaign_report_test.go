package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func campaignReportFixture(t *testing.T, status string) (*Dataset, string) {
	t.Helper()
	dataset, record, attempt := replayEvidence(t)
	dataset.RK[0].Task = "ranking"
	dataset.RK[0].FamilyID = "synthetic-ranking"
	record.RunID = "campaign-run"
	record.Commit = "source-commit"
	record.Plan.Suite = "smoke"
	record.Plan.MaxRetries = 0
	record.Plan.MaximumRequests = 1
	record.Plan.RequestLimit = 1
	if status != "executed" {
		record.Status = "partial"
		attempt = Attempt{CaseID: attempt.CaseID, Trial: attempt.Trial, Status: status, Error: "bounded execution stopped"}
	} else {
		attempt.ProviderResponse = json.RawMessage(`{"modelVersion":"resolved-model","usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3,"totalTokenCount":15}}`)
		attempt.StartedOn = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
		attempt.LatencyMillis = 25
	}
	return dataset, persistReplayEvidence(t, record, attempt)
}

func campaignReportSpec(t *testing.T, dataset *Dataset, runDirectory string) CampaignReportSpec {
	t.Helper()
	record, _, err := ReadRun(runDirectory)
	require.NoError(t, err)
	return CampaignReportSpec{
		FormatVersion:         "1",
		CampaignID:            "campaign-1",
		DatasetVersion:        dataset.Version,
		DatasetManifestSHA256: dataset.ManifestSHA256,
		SourceCommit:          "source-commit",
		ProtocolSHA256:        "protocol-hash",
		Execution:             CampaignExecutionSpec{MaxRetries: record.Plan.MaxRetries, Limits: record.Limits},
		PricingVerifiedOn:     "2026-09-14",
		BaselineFilesSHA256:   map[string]string{"baseline.go": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Budget: CampaignBudgetEvidence{HardCeilingUSD: 10, ExpectedGenerationCalls: 1,
			ObservedGenerationCalls: boolInt(record.Status == "completed"), GuardRecordedUsageCalls: boolInt(record.Status == "completed"), ProviderUsageComplete: true},
		Phases: []CampaignPhaseSpec{{
			Name:   "smoke",
			Suite:  "smoke",
			Trials: 1,
			Runs: []CampaignRunSpec{{
				RequestedModel: "model-a",
				RunDirectory:   runDirectory,
			}},
		}},
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestBuildCampaignReportPreservesDenominatorsAndProvenance(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "executed")
	report, err := BuildCampaignReport(context.Background(), dataset, campaignReportSpec(t, dataset, runDirectory))
	require.NoError(t, err)
	require.Equal(t, "campaign-1", report.Campaign.ID)
	require.Equal(t, "protocol-hash", report.Campaign.ProtocolSHA256)
	require.Len(t, report.Phases, 1)
	model := report.Phases[0].Models[0]
	require.Equal(t, 1, model.Coverage.ExpectedSlots)
	require.Equal(t, 1, model.Coverage.TerminalSlots)
	require.Equal(t, 1, model.Coverage.ExecutedSlots)
	require.True(t, model.Coverage.EvidenceComplete)
	require.True(t, model.Coverage.ResponsesComplete)
	require.Equal(t, 1, model.Coverage.SemanticUnknownSlots)
	require.False(t, model.Coverage.SemanticsComplete)
	require.Equal(t, []string{"resolved-model"}, model.Provenance.ResolvedModelVersions)
	require.Zero(t, model.Provenance.UnknownResolvedModelRequests)
	require.NotEmpty(t, model.Provenance.PromptSHA256)
	require.Nil(t, model.Operations.Cost)
	require.Equal(t, int64(12), *model.Operations.InputTokens.ObservedTotal)
	require.Equal(t, 1.0, *model.Tasks["ranking"].Metrics["ndcg_at_3"].Mean)
	require.Equal(t, 1, model.Tasks["ranking"].TrialVariability["ndcg_at_3"].ExpectedBaseCases)
	require.Zero(t, report.Offline.RankingModelRequests)
	require.Len(t, report.Cases, 1)
	require.NotEmpty(t, report.Cases[0].OutputSHA256)
	require.Equal(t, "unassessed", report.Cases[0].SemanticStatus)
	require.False(t, report.ReleaseApproved)
}

func TestBuildCampaignReportLabelsAgentReviewWithoutHumanCertification(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "executed")
	document, err := NewSemanticReviewTemplate(dataset, runDirectory)
	require.NoError(t, err)
	now := time.Date(2026, 9, 14, 11, 0, 0, 0, time.UTC)
	document.Reviews[0].Result = "pass"
	document.Reviews[0].Evidence = "Reason is grounded in the supplied provider facts."
	document.Reviews[0].Reviewer = "campaign-review-agent"
	document.Reviews[0].ReviewerKind = "agent"
	document.Reviews[0].ReviewedOn = &now
	reviewsPath := filepath.Join(t.TempDir(), "reviews.json")
	require.NoError(t, WriteSemanticReviews(reviewsPath, document))
	spec := campaignReportSpec(t, dataset, runDirectory)
	spec.Phases[0].Runs[0].SemanticReviews = reviewsPath

	report, err := BuildCampaignReport(context.Background(), dataset, spec)
	require.NoError(t, err)
	model := report.Phases[0].Models[0]
	require.Zero(t, model.Coverage.SemanticUnknownSlots)
	require.True(t, model.Coverage.SemanticsComplete)
	require.False(t, model.Coverage.HumanReviewComplete)
	require.Equal(t, 1, model.Coverage.AgentReviewedSlots)
	require.Equal(t, []string{"agent"}, model.Semantic.ReviewerKinds)
	require.Equal(t, 1, model.Semantic.PendingHumanChecks)
	require.NotEmpty(t, model.Semantic.ReviewDocumentSHA256)
	require.False(t, report.ReleaseApproved)
}

func TestBuildCampaignReportMakesExplicitUnexecutedSlotsVisible(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "not_executed")
	report, err := BuildCampaignReport(context.Background(), dataset, campaignReportSpec(t, dataset, runDirectory))
	require.NoError(t, err)
	coverage := report.Phases[0].Models[0].Coverage
	require.Equal(t, 1, coverage.ExpectedSlots)
	require.Equal(t, 1, coverage.TerminalSlots)
	require.Equal(t, 1, coverage.ExecutionFailedSlots)
	require.True(t, coverage.EvidenceComplete)
	require.False(t, coverage.ResponsesComplete)
	require.False(t, coverage.SemanticsComplete)
	require.False(t, report.Status.ResponsesComplete)
	require.Equal(t, "not_executed", report.Cases[0].ExecutionStatus)
}

func TestBuildCampaignReportRejectsUndeclaredOrUnsafeEvidence(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "executed")
	base := campaignReportSpec(t, dataset, runDirectory)
	tests := []struct {
		name   string
		mutate func(*CampaignReportSpec)
		want   string
	}{
		{"dataset", func(s *CampaignReportSpec) { s.DatasetManifestSHA256 = "other" }, "dataset"},
		{"source", func(s *CampaignReportSpec) { s.SourceCommit = "other" }, "source commit"},
		{"trials", func(s *CampaignReportSpec) { s.Phases[0].Trials = 2 }, "trials"},
		{"model", func(s *CampaignReportSpec) { s.Phases[0].Runs[0].RequestedModel = "other" }, "model"},
		{"duplicate model", func(s *CampaignReportSpec) { s.Phases[0].Runs = append(s.Phases[0].Runs, s.Phases[0].Runs[0]) }, "duplicate"},
		{"duplicate phase", func(s *CampaignReportSpec) { s.Phases = append(s.Phases, s.Phases[0]) }, "duplicate"},
		{"holdout", func(s *CampaignReportSpec) { s.Phases[0].Suite = "holdout" }, "holdout"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := base
			spec.Phases = append([]CampaignPhaseSpec(nil), base.Phases...)
			spec.Phases[0].Runs = append([]CampaignRunSpec(nil), base.Phases[0].Runs...)
			tc.mutate(&spec)
			_, err := BuildCampaignReport(context.Background(), dataset, spec)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestReadCampaignReportSpecIsStrictAndHashesExactProtocol(t *testing.T) {
	dataset, _ := campaignReportFixture(t, "executed")
	dataset.Suites["development"] = append([]string(nil), dataset.Suites["smoke"]...)
	protocol := fmt.Sprintf(`{"protocol_version":"1.0.0","campaign_id":"campaign-1","status":"frozen","purpose":"test","dataset_version":%q,"dataset_manifest_sha256":%q,"scope":"development","expected_trials":1,"allow_holdout":false,"dataset":{},"solution":{"baseline_source_commit":"base","baseline_files":{"file":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"require_clean_tree_before_live":true,"prompt_change_allowed":false,"generation_config_change_allowed":false,"quality_fixes_allowed":false},"models":[{"requested_model":"model-a"}],"execution":{"concurrency":1,"max_retries":0,"retry_for_quality":false,"attempt_timeout_seconds":1,"global_timeout_seconds":2,"min_interval_seconds":1,"max_output_tokens":4096,"primary":{"suite":"development","cases":1,"trials_per_case":1,"calls_total":1},"smoke":{"suite":"smoke","cases":1,"trials_per_case":1,"calls_total":1},"maximum_provider_calls_total":2,"contracts":{},"ranking_baselines":{},"metamorphic":{"enabled":false}},"budget":{"currency":"USD","hard_ceiling":10,"guard_required_before_live":true,"guard_policy":"test","max_input_tokens_per_request":1,"max_output_tokens_per_request":4096,"preflight_upper_bound_usd":1,"preflight_calculation":"test","preflight_headroom_usd":9,"pricing":{"source":"pricing-source","must_verify_on_execution_date":true,"model-a":{"input_usd_per_million_tokens":0.3,"output_usd_per_million_tokens":2.5}},"unknown_usage_policy":"fail"},"measurement":{},"traceability":{}}`, dataset.Version, dataset.ManifestSHA256)
	root := t.TempDir()
	protocolPath := filepath.Join(root, "protocol.json")
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocol), 0600))
	evidence := fmt.Sprintf(`{"format_version":"1","campaign_id":"campaign-1","protocol_sha256":%q,"dataset_manifest_sha256":%q,"execution_source_commit":"commit","pricing_verified_on":"2026-09-14","baseline_files_sha256":{"file":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"budget":{"pricing_source":"pricing-source","hard_ceiling_usd":10,"count_token_requests":2,"expected_generation_calls":2,"preflight_input_tokens":2,"reserved_input_tokens":40000,"reserved_output_tokens":8192,"reserved_input_usd":0.012,"reserved_output_usd":0.02048,"reserved_total_usd":0.03248,"preflight_headroom_usd":9.96752,"observed_generation_calls":0,"guard_recorded_usage_calls":0,"accounted_spend_usd":0,"provider_usage_complete":true},"phases":[{"name":"smoke","runs":[{"requested_model":"model-a","run_directory":"smoke-run"}]},{"name":"primary","runs":[{"requested_model":"model-a","run_directory":"primary-run"}]}]}`, digest([]byte(protocol)), dataset.ManifestSHA256)
	evidencePath := filepath.Join(root, "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(evidence), 0600))
	spec, err := ReadCampaignReportSpec(dataset, protocolPath, evidencePath)
	require.NoError(t, err)
	require.Equal(t, digest([]byte(protocol)), spec.ProtocolSHA256)
	require.Equal(t, filepath.Join(root, "smoke-run"), spec.Phases[0].Runs[0].RunDirectory)
	for _, invalid := range []string{evidence + `{}`, `{"unknown":true}`} {
		require.NoError(t, os.WriteFile(evidencePath, []byte(invalid), 0600))
		_, err = ReadCampaignReportSpec(dataset, protocolPath, evidencePath)
		require.Error(t, err)
	}
}

func TestReadCampaignReportSpecAcceptsFrozenCampaignProtocol(t *testing.T) {
	protocolPath := filepath.Join("..", "..", "evals", "protocols", "campaign-1.json")
	dataset, err := LoadDataset(filepath.Join("..", "..", "evals", "datasets", "LoResuelvo_US60_evals_v1.0.0"))
	require.NoError(t, err)
	protocol, err := os.ReadFile(protocolPath)
	require.NoError(t, err)
	models := []string{"gemini-3.1-flash-lite", "gemini-3.5-flash-lite"}
	phase := func(name string) CampaignEvidencePhase {
		result := CampaignEvidencePhase{Name: name}
		for _, model := range models {
			result.Runs = append(result.Runs, CampaignRunSpec{RequestedModel: model, RunDirectory: name + "-" + model})
		}
		return result
	}
	evidence := CampaignEvidence{FormatVersion: "1", CampaignID: "campaign-1", ProtocolSHA256: digest(protocol), DatasetManifestSHA256: dataset.ManifestSHA256, ExecutionSourceCommit: "future-clean-commit", PricingVerifiedOn: "2026-09-14", BaselineFilesSHA256: map[string]string{
		"internal/adapters/chatbot/gemini_chatbot.go":         "865d4bac8787cb774e0d6575671710427115154a355585f0989277aebc4301c0",
		"internal/adapters/chatbot/gemini_generation.go":      "7ed4dc97f4dc1ab3b3471190212cd752dc90ebac7323d38794c64461d061ce73",
		"internal/adapters/chatbot/gemini_chatbot_test.go":    "400b70ab7d7db43c94dcf4cdd91cc310478452b85e316ccb240d33a38f1119f6",
		"internal/adapters/chatbot/gemini_generation_test.go": "fded926bf129f5ac3a9c3454d8388f8c48b4aca716eb28b9c0fd4c6659bed83d",
	}, Budget: CampaignBudgetEvidence{PricingSource: "https://ai.google.dev/gemini-api/docs/pricing", HardCeilingUSD: 10, CountTokenRequests: 144, ExpectedGenerationCalls: 360, ReservedInputTokens: 360 * 20000, ReservedOutputTokens: 360 * 4096, ReservedInputUSD: 2.16, ReservedOutputUSD: 3.6864, ReservedTotalUSD: 5.8464, PreflightHeadroomUSD: 4.1536, ProviderUsageComplete: true}, Phases: []CampaignEvidencePhase{phase("smoke"), phase("primary")}}
	data, err := json.Marshal(evidence)
	require.NoError(t, err)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, data, 0600))

	spec, err := ReadCampaignReportSpec(dataset, protocolPath, evidencePath)
	require.NoError(t, err)
	require.Len(t, spec.Phases, 2)
	require.Equal(t, 1, spec.Phases[0].Trials)
	require.Equal(t, 3, spec.Phases[1].Trials)
	require.Len(t, spec.Phases[1].Runs, 2)
	require.False(t, spec.Phases[1].Metamorphic)
	require.Equal(t, "future-clean-commit", spec.SourceCommit)
	execution, err := ReadCampaignExecutionConfig(dataset, protocolPath)
	require.NoError(t, err)
	require.Equal(t, 360, execution.MaximumGenerationCalls)
	require.Equal(t, 10.0, execution.HardCeilingUSD)
	require.Len(t, execution.Models, 2)
	require.Len(t, execution.Prices, 2)
	for _, price := range execution.Prices {
		require.False(t, price.Verified)
	}
}

func TestCampaignTrialVariabilityUsesWithinCaseRanges(t *testing.T) {
	cases := []CampaignCaseTrial{
		{Task: "ranking", CaseID: "RK-1", Metrics: map[string]any{"ndcg_at_3": 1.0}},
		{Task: "ranking", CaseID: "RK-1", Metrics: map[string]any{"ndcg_at_3": 0.5}},
		{Task: "ranking", CaseID: "RK-1", Metrics: map[string]any{"ndcg_at_3": 0.75}},
		{Task: "ranking", CaseID: "RK-2", Metrics: map[string]any{}},
	}
	result := campaignTrialVariability(cases, "ranking")["ndcg_at_3"]
	require.Equal(t, 2, result.ExpectedBaseCases)
	require.Equal(t, 1, result.EvaluatedBaseCases)
	require.Equal(t, 1, result.ComparableBaseCases)
	require.Equal(t, 1, result.VariableBaseCases)
	require.Equal(t, 0.5, *result.MeanWithinCaseRange)
	require.Equal(t, 0.5, *result.MaxWithinCaseRange)
}
