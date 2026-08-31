package identityverification

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/google/uuid"
)

type ProviderFinder interface {
	FindProviderByAuthID(authID string) (*provider.Provider, error)
}

type VerificationRepository interface {
	Save(ctx context.Context, verification *IdentityVerification) error
	FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*IdentityVerification, error)
	FindLatestByProviderID(ctx context.Context, providerID int) (*IdentityVerification, error)
	FindByProviderID(ctx context.Context, providerID int) ([]IdentityVerification, error)
}

type TransactionalStore interface {
	SaveVerification(ctx context.Context, verification *IdentityVerification) error
	FindVerificationBySessionID(ctx context.Context, sessionID uuid.UUID) (*IdentityVerification, error)
	SaveEvent(ctx context.Context, event *VerificationEvent) (bool, error)
}

type UnitOfWork interface {
	Execute(ctx context.Context, operation func(TransactionalStore) error) error
}
