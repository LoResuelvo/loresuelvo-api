package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
	require.Contains(t, bundle.Manifest.MissingData, "human_review")
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

func TestBuildCampaignExportRejectsMissingPhysicalResponse(t *testing.T) {
	dataset, runDirectory := campaignReportFixture(t, "not_executed")
	reportSpec := campaignReportSpec(t, dataset, runDirectory)
	protocolPath := filepath.Join(t.TempDir(), "protocol.json")
	protocol := []byte(`{"protocol":"fixture"}`)
	require.NoError(t, os.WriteFile(protocolPath, protocol, 0600))
	reportSpec.ProtocolSHA256 = digest(protocol)
	evidencePath := filepath.Join(t.TempDir(), "evidence.json")
	require.NoError(t, os.WriteFile(evidencePath, []byte(`{"evidence":"fixture"}`), 0600))

	_, err := BuildCampaignExport(context.Background(), dataset, CampaignExportSpec{
		Report: reportSpec, ProtocolPath: protocolPath, EvidencePath: evidencePath,
		ExporterSourceCommit: "exporter-commit",
	})
	require.ErrorContains(t, err, "physical response")
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

func TestBuildCampaignOneExportFromExternalEvidence(t *testing.T) {
	evidencePath := os.Getenv("CAMPAIGN_ONE_EVIDENCE")
	if evidencePath == "" {
		t.Skip("CAMPAIGN_ONE_EVIDENCE is only available in the local evidence archive")
	}
	root := filepath.Join("..", "..", "evals")
	dataset, err := LoadDataset(filepath.Join(root, "datasets", "LoResuelvo_US60_evals_v1.0.0"))
	require.NoError(t, err)
	protocolPath := filepath.Join(root, "protocols", "campaign-1.json")
	reportSpec, err := ReadCampaignReportSpec(dataset, protocolPath, evidencePath)
	require.NoError(t, err)
	bundle, err := BuildCampaignExport(context.Background(), dataset, CampaignExportSpec{
		Report: reportSpec, ProtocolPath: protocolPath, EvidencePath: evidencePath,
		RecoveryAddendumPath: filepath.Join(root, "protocols", "campaign-1-recovery-addendum.json"),
		ExporterSourceCommit: "local-verification",
	})
	require.NoError(t, err)
	require.Equal(t, 392, bundle.Manifest.Counts.ProviderAttempts)
	require.Equal(t, 360, bundle.Manifest.Counts.Responses)
	require.Equal(t, 360, bundle.Manifest.Counts.Results)
	require.Equal(t, 4, bundle.Manifest.Counts.MalformedResponses)
	require.Equal(t, 2040, bundle.Manifest.Counts.EffectiveReviews)
	require.Equal(t, 2031, bundle.Manifest.Counts.AgentAssessments)
	require.Zero(t, bundle.Manifest.Counts.HumanAssessments)
	require.Equal(t, 9, bundle.Manifest.Counts.UnassessedReviews)
	recovered := 0
	for _, response := range bundle.Responses {
		if response.Recovered {
			recovered++
		}
	}
	require.Equal(t, 21, recovered)
	resolved, unknown := 0, 0
	for _, model := range bundle.Manifest.Models {
		unknown += model.UnknownAttempts
		for _, count := range model.ResolvedModelOccurrences {
			resolved += count
		}
	}
	require.Equal(t, 360, resolved)
	require.Equal(t, 32, unknown)
}
