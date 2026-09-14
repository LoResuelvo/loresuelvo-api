package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/require"
)

func TestWriteCampaignExportCreatesLongitudinalDataPackageWithoutCharts(t *testing.T) {
	raw := `{"status":"ok"}`
	bundle := evals.CampaignExport{
		Manifest: evals.CampaignExportManifest{
			FormatVersion: "1", CampaignID: "campaign-1", Artifacts: map[string]evals.CampaignExportArtifact{},
			Counts:      evals.CampaignExportCounts{PlannedSlots: 1, ProviderAttempts: 1, Responses: 1, Results: 1},
			PrivacyScan: evals.CampaignExportPrivacyScan{Status: "passed"},
		},
		Attempts:  []evals.CampaignExportAttempt{{AttemptID: "attempt-1", SlotID: "slot-1"}},
		Responses: []evals.CampaignExportResponse{{ResponseID: "response-1", SlotID: "slot-1", RawOutput: &raw, ParsedOutput: json.RawMessage(raw)}},
		Results:   []evals.CampaignExportResult{{SlotID: "slot-1", ResponseID: "response-1"}},
		Reviews:   []evals.CampaignExportReview{},
		Summary:   evals.CampaignExportSummary{FormatVersion: "1", Campaign: evals.CampaignProvenance{ID: "campaign-1"}},
	}
	directory := filepath.Join(t.TempDir(), "campaign-1")
	require.NoError(t, writeCampaignExport(directory, bundle))
	for _, name := range []string{"README.md", "manifest.json", "attempts.jsonl", "responses.jsonl", "results.jsonl", "reviews.jsonl", "summary.json", "SHA256SUMS"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		require.NoError(t, err, name)
		require.NotNil(t, data)
	}
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotEqual(t, ".svg", filepath.Ext(entry.Name()))
	}
	responseData, err := os.ReadFile(filepath.Join(directory, "responses.jsonl"))
	require.NoError(t, err)
	var response evals.CampaignExportResponse
	require.NoError(t, json.Unmarshal(responseData, &response))
	require.Equal(t, raw, *response.RawOutput)

	manifestData, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	require.NoError(t, err)
	var manifest evals.CampaignExportManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	require.Contains(t, manifest.Artifacts, "responses.jsonl")
	require.Equal(t, 1, *manifest.Artifacts["responses.jsonl"].Records)
	require.NotContains(t, string(manifestData), directory)
	checksums, err := os.ReadFile(filepath.Join(directory, "SHA256SUMS"))
	require.NoError(t, err)
	require.Contains(t, string(checksums), "  manifest.json\n")
	require.False(t, strings.Contains(string(checksums), "SHA256SUMS"))
	require.Error(t, writeCampaignExport(directory, bundle))
}

func TestRenderCampaignExportRejectsSecretOrImagePayload(t *testing.T) {
	raw := `iVBORw0KGgoAAA`
	bundle := evals.CampaignExport{
		Manifest:  evals.CampaignExportManifest{Artifacts: map[string]evals.CampaignExportArtifact{}},
		Responses: []evals.CampaignExportResponse{{RawOutput: &raw}},
	}
	_, err := renderCampaignExportArtifacts(bundle)
	require.ErrorContains(t, err, "image payload")
}

func TestRenderCampaignOneExternalEvidencePassesPublicationSafety(t *testing.T) {
	evidencePath := os.Getenv("CAMPAIGN_ONE_EVIDENCE")
	if evidencePath == "" {
		t.Skip("CAMPAIGN_ONE_EVIDENCE is only available in the local evidence archive")
	}
	root := filepath.Join("..", "..", "evals")
	dataset, err := evals.LoadDataset(filepath.Join(root, "datasets", "LoResuelvo_US60_evals_v1.0.0"))
	require.NoError(t, err)
	protocolPath := filepath.Join(root, "protocols", "campaign-1.json")
	reportSpec, err := evals.ReadCampaignReportSpec(dataset, protocolPath, evidencePath)
	require.NoError(t, err)
	bundle, err := evals.BuildCampaignExport(context.Background(), dataset, evals.CampaignExportSpec{
		Report: reportSpec, ProtocolPath: protocolPath, EvidencePath: evidencePath,
		RecoveryAddendumPath: filepath.Join(root, "protocols", "campaign-1-recovery-addendum.json"),
		ExporterSourceCommit: "local-verification",
	})
	require.NoError(t, err)
	artifacts, err := renderCampaignExportArtifacts(bundle)
	require.NoError(t, err)
	require.Len(t, artifacts, 8)
	require.Equal(t, 392, strings.Count(string(artifacts["attempts.jsonl"]), "\n"))
	require.Equal(t, 360, strings.Count(string(artifacts["responses.jsonl"]), "\n"))
	require.Equal(t, 360, strings.Count(string(artifacts["results.jsonl"]), "\n"))
	require.Equal(t, 2190, strings.Count(string(artifacts["reviews.jsonl"]), "\n"))
	require.NotContains(t, string(artifacts["summary.json"]), "case_trials")
	for name, data := range artifacts {
		require.NotContains(t, string(data), "/home/joseph/", name)
		require.NotContains(t, string(data), "iVBORw0KGgo", name)
	}
}
