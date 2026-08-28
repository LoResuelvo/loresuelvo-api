package identityverification

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/google/uuid"
)

// ProviderFinder is the smallest capability needed to resolve the authenticated
// provider before starting a verification.
type ProviderFinder interface {
	FindProviderByAuthID(ctx context.Context, authID string) (*provider.Provider, error)
}

type VerificationRepository interface {
	Save(ctx context.Context, verification *IdentityVerification) error
	FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*IdentityVerification, error)
	FindLatestByProviderID(ctx context.Context, providerID int) (*IdentityVerification, error)
}

type VerificationEvent struct {
	ExternalEventID   uuid.UUID
	ExternalSessionID uuid.UUID
	EventType         string
	OccurredOn        time.Time
	ReceivedOn        time.Time
}

type VerificationEventStore interface {
	EventExists(ctx context.Context, eventID uuid.UUID) (bool, error)
	SaveEvent(ctx context.Context, event VerificationEvent) error
}

// IdentityVerificationUnitOfWork persists an event and its aggregate in one
// transaction. Implementations treat duplicate event IDs as idempotent.
type IdentityVerificationUnitOfWork interface {
	SaveResult(ctx context.Context, event VerificationEvent, verification *IdentityVerification) error
}

type StatusSnapshot struct {
	Status     VerificationStatus
	Verified   bool
	VerifiedOn *time.Time
	RiskCodes  []string
}

type IdentityVerificationStatusReader interface {
	FindStatusByProviderID(ctx context.Context, providerID int) (StatusSnapshot, error)
}
