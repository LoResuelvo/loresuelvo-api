package didit

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
)

const webhookTypeStatusUpdated = "status.updated"

type WebhookAdapter struct {
	secret string
}

type webhookPayload struct {
	EventID         uuid.UUID `json:"event_id"`
	RiskCodes       []string  `json:"risk_codes"`
	SessionID       uuid.UUID `json:"session_id"`
	Status          string    `json:"status"`
	WebhookType     string    `json:"webhook_type"`
	CreatedAt       int64     `json:"created_at"`
	Timestamp       int64     `json:"timestamp"`
	WorkflowID      uuid.UUID `json:"workflow_id"`
	WorkflowVersion int       `json:"workflow_version"`
	VendorData      string    `json:"vendor_data"`
}

func NewWebhookAdapter(secret string) (*WebhookAdapter, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, identityverification.ErrVerifierMisconfigured
	}
	return &WebhookAdapter{secret: secret}, nil
}

func (adapter *WebhookAdapter) Authenticate(body []byte, signature, timestamp string, now time.Time) error {
	if len(body) == 0 || strings.TrimSpace(signature) == "" || strings.TrimSpace(timestamp) == "" || now.IsZero() {
		return identityverification.ErrInvalidVerification
	}
	dispatchedAt, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil || dispatchedAt < now.Unix()-300 || dispatchedAt > now.Unix()+300 {
		return identityverification.ErrInvalidVerification
	}
	canonical, err := canonicalJSON(body)
	if err != nil {
		return identityverification.ErrInvalidVerification
	}
	mac := hmac.New(sha256.New, []byte(adapter.secret))
	_, err = mac.Write(canonical)
	if err != nil {
		return fmt.Errorf("signing Didit webhook payload: %w", err)
	}
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil || !hmac.Equal(mac.Sum(nil), provided) {
		return identityverification.ErrInvalidVerification
	}
	return nil
}

func (adapter *WebhookAdapter) Translate(body []byte) (identityverification.VerificationResult, error) {
	var payload webhookPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return identityverification.VerificationResult{}, identityverification.ErrInvalidVerification
	}
	if payload.EventID == uuid.Nil || payload.SessionID == uuid.Nil || payload.WorkflowID == uuid.Nil || payload.WorkflowVersion < 0 || payload.CreatedAt <= 0 || payload.Timestamp <= 0 || payload.WebhookType != webhookTypeStatusUpdated {
		return identityverification.VerificationResult{}, identityverification.ErrInvalidVerification
	}
	if strings.TrimSpace(payload.VendorData) == "" {
		return identityverification.VerificationResult{}, identityverification.ErrInvalidVerification
	}
	providerID, err := strconv.Atoi(strings.TrimSpace(payload.VendorData))
	if err != nil || providerID <= 0 {
		return identityverification.VerificationResult{}, identityverification.ErrInvalidVerification
	}
	status, ok := statusFromDidit(payload.Status)
	if !ok {
		return identityverification.VerificationResult{}, identityverification.ErrInvalidVerification
	}
	return identityverification.VerificationResult{
		EventID: payload.EventID, SessionID: payload.SessionID, ProviderID: providerID, VendorData: payload.VendorData, WorkflowID: payload.WorkflowID,
		WorkflowVersion: payload.WorkflowVersion, Status: status, RiskCodes: payload.RiskCodes,
		OccurredOn: time.Unix(payload.CreatedAt, 0).UTC(),
	}, nil
}

func statusFromDidit(status string) (identityverification.VerificationStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "in progress", "in_progress":
		return identityverification.StatusInProgress, true
	case "awaiting user", "awaiting_user":
		return identityverification.StatusAwaitingUser, true
	case "in review", "in_review":
		return identityverification.StatusInReview, true
	case "approved":
		return identityverification.StatusApproved, true
	case "declined":
		return identityverification.StatusDeclined, true
	case "resubmitted", "re-submitted":
		return identityverification.StatusResubmitted, true
	case "abandoned":
		return identityverification.StatusAbandoned, true
	case "expired":
		return identityverification.StatusExpired, true
	case "kyc expired", "kyc_expired":
		return identityverification.StatusKYCExpired, true
	}
	return "", false
}

func canonicalJSON(body []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	value = normalizeJSONValue(value)
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = normalizeJSONValue(child)
		}
	case []any:
		for index, child := range typed {
			typed[index] = normalizeJSONValue(child)
		}
	case json.Number:
		parsed, err := strconv.ParseFloat(string(typed), 64)
		if err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && math.Trunc(parsed) == parsed && parsed >= math.MinInt64 && parsed <= math.MaxInt64 {
			return int64(parsed)
		}
	}
	return value
}
