package didit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWebhookAdapterTranslatesInProgressStatus(t *testing.T) {
	sessionID, eventID, workflowID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `","session_id":"` + sessionID.String() + `","status":"In Progress","webhook_type":"status.updated","created_at":1788264000,"timestamp":1788264000,"workflow_id":"` + workflowID.String() + `","workflow_version":2,"vendor_data":"42"}`)
	adapter, err := NewWebhookAdapter("webhook-secret")
	require.NoError(t, err)

	result, err := adapter.Translate(body)

	require.NoError(t, err)
	require.Equal(t, eventID, result.EventID)
	require.Equal(t, sessionID, result.SessionID)
	require.Equal(t, identityverification.StatusInProgress, result.Status)
	require.Equal(t, 42, result.ProviderID)
	require.Equal(t, workflowID, result.WorkflowID)
}

func TestWebhookAdapterTranslatesAwaitingUserStatus(t *testing.T) {
	sessionID, eventID, workflowID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `","session_id":"` + sessionID.String() + `","status":"Awaiting User","webhook_type":"status.updated","created_at":1788264000,"timestamp":1788264000,"workflow_id":"` + workflowID.String() + `","workflow_version":2,"vendor_data":"42"}`)
	adapter, err := NewWebhookAdapter("webhook-secret")
	require.NoError(t, err)

	result, err := adapter.Translate(body)

	require.NoError(t, err)
	require.Equal(t, sessionID, result.SessionID)
	require.Equal(t, identityverification.StatusAwaitingUser, result.Status)
}

func TestWebhookAdapterTranslatesInReviewStatus(t *testing.T) {
	sessionID, eventID, workflowID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `","session_id":"` + sessionID.String() + `","status":"In Review","webhook_type":"status.updated","created_at":1788264000,"timestamp":1788264000,"workflow_id":"` + workflowID.String() + `","workflow_version":2,"vendor_data":"42"}`)
	adapter, err := NewWebhookAdapter("webhook-secret")
	require.NoError(t, err)

	result, err := adapter.Translate(body)

	require.NoError(t, err)
	require.Equal(t, sessionID, result.SessionID)
	require.Equal(t, identityverification.StatusInReview, result.Status)
}

func TestWebhookAdapterTranslatesTerminalStatuses(t *testing.T) {
	statuses := map[string]identityverification.VerificationStatus{
		"Approved":    identityverification.StatusApproved,
		"Declined":    identityverification.StatusDeclined,
		"Resubmitted": identityverification.StatusResubmitted,
		"Abandoned":   identityverification.StatusAbandoned,
		"Expired":     identityverification.StatusExpired,
		"KYC Expired": identityverification.StatusKYCExpired,
	}
	adapter, err := NewWebhookAdapter("webhook-secret")
	require.NoError(t, err)

	for diditStatus, expectedStatus := range statuses {
		t.Run(diditStatus, func(t *testing.T) {
			sessionID, eventID, workflowID := uuid.New(), uuid.New(), uuid.New()
			body := []byte(`{"event_id":"` + eventID.String() + `","session_id":"` + sessionID.String() + `","status":"` + diditStatus + `","webhook_type":"status.updated","created_at":1788264000,"timestamp":1788264000,"workflow_id":"` + workflowID.String() + `","workflow_version":2,"vendor_data":"42"}`)

			result, err := adapter.Translate(body)

			require.NoError(t, err)
			require.Equal(t, expectedStatus, result.Status)
		})
	}
}

func TestWebhookAdapterTranslatesRiskCodes(t *testing.T) {
	sessionID, eventID, workflowID := uuid.New(), uuid.New(), uuid.New()
	body := []byte(`{"event_id":"` + eventID.String() + `","session_id":"` + sessionID.String() + `","status":"Declined","risk_codes":["DOCUMENT_EXPIRED","DOCUMENT_MISMATCH"],"webhook_type":"status.updated","created_at":1788264000,"timestamp":1788264000,"workflow_id":"` + workflowID.String() + `","workflow_version":2,"vendor_data":"42"}`)
	adapter, err := NewWebhookAdapter("webhook-secret")
	require.NoError(t, err)

	result, err := adapter.Translate(body)

	require.NoError(t, err)
	require.Equal(t, identityverification.StatusDeclined, result.Status)
	require.Equal(t, []string{"DOCUMENT_EXPIRED", "DOCUMENT_MISMATCH"}, result.RiskCodes)
}

func TestWebhookAdapterAuthenticatesCanonicalJSON(t *testing.T) {
	now := time.Unix(1788264000, 0).UTC()
	sessionID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	body := []byte(`{"vendor_data":"José","value":100.0,"session_id":"` + sessionID.String() + `"}`)
	canonical, err := canonicalJSON(body)
	require.NoError(t, err)
	require.Equal(t, `{"session_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","value":100,"vendor_data":"José"}`, string(canonical))
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, err = mac.Write(canonical)
	require.NoError(t, err)

	adapter, err := NewWebhookAdapter("webhook-secret")
	require.NoError(t, err)
	require.NoError(t, adapter.Authenticate(body, hex.EncodeToString(mac.Sum(nil)), "1788264000", now))
	require.ErrorIs(t, adapter.Authenticate(body, "00", "1788264000", now), identityverification.ErrInvalidVerification)
	require.ErrorIs(t, adapter.Authenticate(body, hex.EncodeToString(mac.Sum(nil)), "1788260000", now), identityverification.ErrInvalidVerification)
}
