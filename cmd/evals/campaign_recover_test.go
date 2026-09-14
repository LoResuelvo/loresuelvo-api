package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/require"
)

func TestReadCampaignRecoveryAddendumRejectsUnknownFieldsAndUnsafeScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "addendum.json")
	canonical, err := os.ReadFile(filepath.Join("..", "..", "evals", "protocols", "campaign-1-recovery-addendum.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, canonical, 0600))
	_, err = readCampaignRecoveryAddendum(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"protocol_version":"1.0.0","addendum_id":"r","campaign_id":"campaign-1","parent_protocol":"campaign-1.json","unexpected":true}`), 0600))
	_, err = readCampaignRecoveryAddendum(path)
	require.Error(t, err)
}

func TestRecoverySelectionNeverIncludesContentOrNonTransientErrors(t *testing.T) {
	base := evals.Attempt{CaseID: "PD-001", Trial: 1, Retry: 0, Status: "execution_error", Error: "503 UNAVAILABLE", RequestCount: 1}
	require.Equal(t, "transient_provider_unavailable", recoveryTransientReason(base))
	base.RawOutput = `{"malformed":`
	require.Empty(t, recoveryTransientReason(base))
	base.RawOutput = ""
	base.Error = "invalid JSON response"
	require.Empty(t, recoveryTransientReason(base))
	base.Error = "context deadline exceeded"
	require.Equal(t, "transient_request_timeout", recoveryTransientReason(base))
}

func TestCampaignRecoveryUpperBoundUsesHighestConfiguredPrice(t *testing.T) {
	config := evals.CampaignExecutionConfig{Prices: []evals.CampaignPrice{{InputUSDPerMillion: .25, OutputUSDPerMillion: 1.5}, {InputUSDPerMillion: .3, OutputUSDPerMillion: 2.5}}}
	require.InDelta(t, .01624, campaignRecoveryUpperBound(1, config), .000001)
	require.InDelta(t, .06496, campaignRecoveryUpperBound(4, config), .000001)
}
