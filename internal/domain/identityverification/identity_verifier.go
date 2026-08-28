package identityverification

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SessionRequest struct {
	ProviderID        int
	VendorData        string
	FirstName         string
	LastName          string
	ExistingSessionID *uuid.UUID
}

type SessionCredentials struct {
	SessionID       uuid.UUID
	SessionToken    string
	VerificationURL string
	Status          VerificationStatus
	Verifier        string
	WorkflowID      uuid.UUID
	WorkflowVersion int
}

type IdentityVerifier interface {
	CreateSession(ctx context.Context, request SessionRequest) (SessionCredentials, error)
}

type VerificationResult struct {
	SessionID       uuid.UUID
	ProviderID      int
	VendorData      string
	WorkflowID      uuid.UUID
	WorkflowVersion int
	Status          VerificationStatus
	RiskCodes       []string
	OccurredOn      time.Time
}
