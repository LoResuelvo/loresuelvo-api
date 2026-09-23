package category

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
)

type Repository interface {
	Save(category Category) (*Category, error)
	ListAll() ([]Category, error)
	FindByID(id int) *Category
}

// OperatorIDFinder resolves the authenticated subject to the durable internal ID
// recorded in audit events. Authorization remains the HTTP boundary's concern.
type OperatorIDFinder interface {
	FindOperatorIDByAuthID(ctx context.Context, authID string) (int, error)
}

// TransactionalStore exposes only the writes needed for an atomic category creation.
type TransactionalStore interface {
	SaveCategory(ctx context.Context, category Category) (*Category, error)
	SaveAuditEvent(ctx context.Context, event *audit.Event) error
}

type UnitOfWork interface {
	Execute(ctx context.Context, operation func(TransactionalStore) error) error
}
