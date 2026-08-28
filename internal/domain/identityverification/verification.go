package identityverification

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// IdentityVerification represents one hosted session without temporary credentials or verifier payloads.
type IdentityVerification struct {
	ExternalSessionID uuid.UUID
	ProviderID        int
	Verifier          string
	WorkflowID        uuid.UUID
	WorkflowVersion   int
	Status            VerificationStatus
	RiskCodes         []string
	LastResultOn      *time.Time
	VerifiedOn        *time.Time
	CreatedOn         time.Time
	UpdatedOn         time.Time
}

func NewVerification(providerID int, sessionID, workflowID uuid.UUID, verifier string, workflowVersion int, now time.Time) (*IdentityVerification, error) {
	if providerID <= 0 || sessionID == uuid.Nil || workflowID == uuid.Nil || strings.TrimSpace(verifier) == "" || workflowVersion < 0 || now.IsZero() {
		return nil, ErrInvalidVerification
	}
	now = now.UTC()
	return &IdentityVerification{ExternalSessionID: sessionID, ProviderID: providerID, Verifier: strings.TrimSpace(verifier), WorkflowID: workflowID, WorkflowVersion: workflowVersion, Status: StatusNotStarted, CreatedOn: now, UpdatedOn: now}, nil
}

func Rehydrate(externalSessionID uuid.UUID, providerID int, verifier string, workflowID uuid.UUID, workflowVersion int, status VerificationStatus, createdOn, updatedOn time.Time) (*IdentityVerification, error) {
	return RehydrateWithMetadata(externalSessionID, providerID, verifier, workflowID, workflowVersion, status, createdOn, updatedOn, nil, nil, nil)
}

func RehydrateWithMetadata(externalSessionID uuid.UUID, providerID int, verifier string, workflowID uuid.UUID, workflowVersion int, status VerificationStatus, createdOn, updatedOn time.Time, riskCodes []string, lastResultOn, verifiedOn *time.Time) (*IdentityVerification, error) {
	if (status != StatusNotStarted && status != StatusInProgress && status != StatusAwaitingUser && status != StatusInReview && status != StatusApproved && status != StatusDeclined && status != StatusResubmitted && status != StatusAbandoned && status != StatusExpired && status != StatusKYCExpired) || updatedOn.IsZero() {
		return nil, ErrInvalidVerification
	}
	verification, err := NewVerification(providerID, externalSessionID, workflowID, verifier, workflowVersion, createdOn)
	if err != nil {
		return nil, err
	}
	verification.Status = status
	verification.RiskCodes = sanitizeRiskCodes(riskCodes)
	verification.LastResultOn = copyTime(lastResultOn)
	verification.VerifiedOn = copyTime(verifiedOn)
	verification.UpdatedOn = updatedOn.UTC()
	return verification, nil
}

func (verification *IdentityVerification) ApplyResult(result VerificationResult, now time.Time) error {
	if result.SessionID != verification.ExternalSessionID ||
		result.ProviderID != verification.ProviderID ||
		result.WorkflowID != verification.WorkflowID ||
		result.WorkflowVersion != verification.WorkflowVersion ||
		result.VendorData != ProviderVendorData(verification.ProviderID) ||
		!result.Status.CanApplyResult() ||
		result.OccurredOn.IsZero() || now.IsZero() {
		return ErrInvalidVerification
	}
	verification.Status = result.Status
	resultOn := result.OccurredOn.UTC()
	verification.LastResultOn = &resultOn
	verification.RiskCodes = nil
	verification.VerifiedOn = nil
	if result.Status == StatusApproved {
		verifiedOn := now.UTC()
		verification.VerifiedOn = &verifiedOn
	}
	if result.Status == StatusDeclined {
		verification.RiskCodes = sanitizeRiskCodes(result.RiskCodes)
	}
	verification.UpdatedOn = now.UTC()
	return nil
}

func ProviderVendorData(providerID int) string { return strconv.Itoa(providerID) }

func sanitizeRiskCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	sanitized := make([]string, 0, len(codes))
	for _, code := range codes {
		normalized := strings.ToUpper(strings.TrimSpace(code))
		if normalized == "" || !isRiskCode(normalized) {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		sanitized = append(sanitized, normalized)
	}
	return sanitized
}

func isRiskCode(code string) bool {
	for _, character := range code {
		if (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
