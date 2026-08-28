package identityverification

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIdentityVerificationApplyApprovedResult(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID := uuid.New()
	workflowID := uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)

	resultTime := now.Add(time.Minute)
	err = verification.Apply(VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, WorkflowID: workflowID,
		WorkflowVersion: 1, Status: StatusApproved, OccurredOn: resultTime,
	}, now.Add(2*time.Minute))

	require.NoError(t, err)
	require.Equal(t, StatusApproved, verification.Status)
	require.NotNil(t, verification.VerifiedOn)
	require.Equal(t, resultTime, *verification.VerifiedOn)
}

func TestIdentityVerificationIgnoresOlderResultAndSanitizesRiskCodes(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID := uuid.New()
	workflowID := uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)
	approvedAt := now.Add(time.Minute)
	require.NoError(t, verification.Apply(VerificationResult{
		SessionID: sessionID, Status: StatusApproved, OccurredOn: approvedAt,
	}, approvedAt))

	err = verification.Apply(VerificationResult{
		SessionID: sessionID, Status: StatusInProgress, OccurredOn: now,
		RiskCodes: []string{"document_expired", "contains spaces", "SAFE_CODE"},
	}, approvedAt.Add(time.Minute))

	require.NoError(t, err)
	require.Equal(t, StatusApproved, verification.Status)
	require.Empty(t, verification.RiskCodes)
}

func TestProviderVendorDataUsesInternalID(t *testing.T) {
	require.Equal(t, "42", ProviderVendorData(42))
}
