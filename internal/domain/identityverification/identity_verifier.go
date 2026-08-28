package identityverification

import (
	"context"

	"github.com/google/uuid"
)

type SessionRequest struct {
	ProviderID int
	VendorData string
	FirstName  string
	LastName   string
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
