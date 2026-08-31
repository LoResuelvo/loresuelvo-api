package identityverification

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewIdentityVerificationStartsNotStarted(t *testing.T) {
	verification, err := NewVerification(7, uuid.New(), uuid.New(), "fake", 1, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, StatusNotStarted, verification.Status)
}

func TestProviderVendorDataUsesInternalID(t *testing.T) {
	require.Equal(t, "42", ProviderVendorData(42))
}

func TestIdentityVerificationAppliesApprovedResultAndRecordsVerificationTime(t *testing.T) {
	createdOn := time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)
	verifiedOn := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	resultOn := verifiedOn.Add(-time.Minute)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, createdOn)
	require.NoError(t, err)

	err = verification.ApplyResult(VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusApproved,
		OccurredOn: resultOn,
	}, verifiedOn)

	require.NoError(t, err)
	require.Equal(t, StatusApproved, verification.Status)
	require.NotNil(t, verification.VerifiedOn)
	require.Equal(t, verifiedOn, *verification.VerifiedOn)
	require.NotNil(t, verification.LastResultOn)
	require.Equal(t, resultOn, *verification.LastResultOn)
	require.Empty(t, verification.RiskCodes)
}

func TestIdentityVerificationAppliesDeclinedResultWithSanitizedRiskCodes(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)

	err = verification.ApplyResult(VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusDeclined,
		RiskCodes:  []string{" DOCUMENT_EXPIRED ", "drop table", "DOCUMENT_EXPIRED", "KYC-FAIL"},
		OccurredOn: now.Add(time.Minute),
	}, now.Add(2*time.Minute))

	require.NoError(t, err)
	require.Equal(t, StatusDeclined, verification.Status)
	require.Equal(t, []string{"DOCUMENT_EXPIRED", "KYC-FAIL"}, verification.RiskCodes)
	require.Nil(t, verification.VerifiedOn)
}

func TestIdentityVerificationAppliesResubmittedAbandonedAndExpiredResults(t *testing.T) {
	statuses := []VerificationStatus{StatusResubmitted, StatusAbandoned, StatusExpired}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
			sessionID, workflowID := uuid.New(), uuid.New()
			verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
			require.NoError(t, err)

			err = verification.ApplyResult(VerificationResult{
				EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
				WorkflowID: workflowID, WorkflowVersion: 1, Status: status,
				OccurredOn: now.Add(time.Minute),
			}, now.Add(2*time.Minute))

			require.NoError(t, err)
			require.Equal(t, status, verification.Status)
		})
	}
}

func TestIdentityVerificationClearsVerificationDateWhenKYCExpires(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)
	verification.Status = StatusApproved
	verification.VerifiedOn = &now

	err = verification.ApplyResult(VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusKYCExpired,
		OccurredOn: now.Add(time.Minute),
	}, now.Add(2*time.Minute))

	require.NoError(t, err)
	require.Equal(t, StatusKYCExpired, verification.Status)
	require.Nil(t, verification.VerifiedOn)
}

func TestIdentityVerificationIgnoresResultOlderThanLastAcceptedResult(t *testing.T) {
	approvedOn := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, approvedOn.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, verification.ApplyResult(VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusApproved, OccurredOn: approvedOn,
	}, approvedOn))

	err = verification.ApplyResult(VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusInProgress,
		OccurredOn: approvedOn.Add(-time.Minute),
	}, approvedOn.Add(time.Minute))

	require.NoError(t, err)
	require.Equal(t, StatusApproved, verification.Status)
	require.Equal(t, approvedOn, *verification.LastResultOn)
	require.Equal(t, approvedOn, *verification.VerifiedOn)
	require.Equal(t, approvedOn, verification.UpdatedOn)
}
