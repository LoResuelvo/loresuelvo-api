package identityverification

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxRiskCodes      = 20
	maxRiskCodeLength = 64
)

var safeRiskCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.-]{0,63}$`)

// IdentityVerification is the provider-independent aggregate for one hosted
// identity verification session. It intentionally contains no temporary
// credentials or provider payload.
type IdentityVerification struct {
	ExternalSessionID uuid.UUID
	ProviderID        int
	Verifier          string
	WorkflowID        uuid.UUID
	WorkflowVersion   int
	Status            VerificationStatus
	RiskCodes         []string
	LastResultOn      time.Time
	VerifiedOn        *time.Time
	CreatedOn         time.Time
	UpdatedOn         time.Time
}

func NewVerification(providerID int, sessionID, workflowID uuid.UUID, verifier string, workflowVersion int, now time.Time) (*IdentityVerification, error) {
	if providerID <= 0 || sessionID == uuid.Nil || workflowID == uuid.Nil || strings.TrimSpace(verifier) == "" || workflowVersion < 0 || now.IsZero() {
		return nil, ErrInvalidResult
	}
	return &IdentityVerification{
		ExternalSessionID: sessionID,
		ProviderID:        providerID,
		Verifier:          strings.TrimSpace(verifier),
		WorkflowID:        workflowID,
		WorkflowVersion:   workflowVersion,
		Status:            StatusNotStarted,
		RiskCodes:         nil,
		CreatedOn:         now.UTC(),
		UpdatedOn:         now.UTC(),
	}, nil
}

func Rehydrate(externalSessionID uuid.UUID, providerID int, verifier string, workflowID uuid.UUID, workflowVersion int, status VerificationStatus, riskCodes []string, lastResultOn time.Time, verifiedOn *time.Time, createdOn, updatedOn time.Time) (*IdentityVerification, error) {
	if externalSessionID == uuid.Nil || providerID <= 0 || workflowID == uuid.Nil || strings.TrimSpace(verifier) == "" || !status.Valid() || createdOn.IsZero() || updatedOn.IsZero() {
		return nil, ErrInvalidResult
	}
	if !lastResultOn.IsZero() && lastResultOn.Before(createdOn) {
		return nil, ErrInvalidResult
	}
	codes := sanitizeRiskCodes(riskCodes)
	if status != StatusApproved {
		verifiedOn = nil
	}
	return &IdentityVerification{
		ExternalSessionID: externalSessionID,
		ProviderID:        providerID,
		Verifier:          strings.TrimSpace(verifier),
		WorkflowID:        workflowID,
		WorkflowVersion:   workflowVersion,
		Status:            status,
		RiskCodes:         codes,
		LastResultOn:      lastResultOn.UTC(),
		VerifiedOn:        cloneTime(verifiedOn),
		CreatedOn:         createdOn.UTC(),
		UpdatedOn:         updatedOn.UTC(),
	}, nil
}

// Apply accepts a provider-neutral status result. Older results are ignored
// so an out-of-order webhook cannot revert a newer state.
func (verification *IdentityVerification) Apply(result VerificationResult, now time.Time) error {
	if verification == nil || result.SessionID != verification.ExternalSessionID || !result.Status.Valid() || result.OccurredOn.IsZero() || now.IsZero() {
		return ErrInvalidResult
	}
	if result.ProviderID != 0 && result.ProviderID != verification.ProviderID {
		return ErrInvalidResult
	}
	if result.WorkflowID != uuid.Nil && result.WorkflowID != verification.WorkflowID {
		return ErrInvalidResult
	}
	if result.WorkflowVersion != 0 && result.WorkflowVersion != verification.WorkflowVersion {
		return ErrInvalidResult
	}
	if strings.TrimSpace(result.VendorData) != "" && strings.TrimSpace(result.VendorData) != providerVendorData(verification.ProviderID) {
		return ErrInvalidResult
	}
	if !verification.LastResultOn.IsZero() && !result.OccurredOn.After(verification.LastResultOn) {
		return nil
	}

	verification.Status = result.Status
	verification.RiskCodes = sanitizeRiskCodes(result.RiskCodes)
	verification.LastResultOn = result.OccurredOn.UTC()
	verification.UpdatedOn = now.UTC()
	switch result.Status {
	case StatusApproved:
		verifiedOn := result.OccurredOn.UTC()
		verification.VerifiedOn = &verifiedOn
	default:
		verification.VerifiedOn = nil
	}
	return nil
}

func (verification IdentityVerification) IsApproved() bool {
	return verification.Status == StatusApproved && verification.VerifiedOn != nil
}

func sanitizeRiskCodes(codes []string) []string {
	result := make([]string, 0, min(len(codes), maxRiskCodes))
	for _, code := range codes {
		code = strings.TrimSpace(strings.ToUpper(code))
		if len(code) == 0 || len(code) > maxRiskCodeLength || !safeRiskCode.MatchString(code) || slices.Contains(result, code) {
			continue
		}
		result = append(result, code)
		if len(result) == maxRiskCodes {
			break
		}
	}
	return result
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func providerVendorData(providerID int) string {
	return strconv.Itoa(providerID)
}
