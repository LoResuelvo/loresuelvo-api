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
	if status != StatusNotStarted || updatedOn.IsZero() {
		return nil, ErrInvalidVerification
	}
	verification, err := NewVerification(providerID, externalSessionID, workflowID, verifier, workflowVersion, createdOn)
	if err != nil {
		return nil, err
	}
	verification.UpdatedOn = updatedOn.UTC()
	return verification, nil
}

func ProviderVendorData(providerID int) string { return strconv.Itoa(providerID) }
