package category

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
)

type Repository interface {
	Save(category Category) (*Category, error)
	ListAll() ([]Category, error)
	FindByID(ctx context.Context, id int) (*Category, error)
}

// OperatorIDFinder resolves the authenticated subject to the durable internal ID
// recorded in audit events. Authorization remains the HTTP boundary's concern.
type OperatorIDFinder interface {
	FindOperatorIDByAuthID(ctx context.Context, authID string) (int, error)
}

// TransactionalStore coordinates category state, fresh impact and audit persistence.
type TransactionalStore interface {
	FindCategory(ctx context.Context, id int) (*Category, error)
	FindImpact(ctx context.Context, id int, observedAt time.Time) (*Impact, error)
	SaveCategory(ctx context.Context, category Category) (*Category, error)
	SaveAuditEvent(ctx context.Context, event *audit.Event) error
}

type UnitOfWork interface {
	Execute(ctx context.Context, operation func(TransactionalStore) error) error
}
