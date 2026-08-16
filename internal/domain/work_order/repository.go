package workorder

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Reader interface {
	FindByID(ctx context.Context, id int) (*WorkOrder, error)
	FindByUserID(ctx context.Context, userID int, viewerRole string) ([]readmodel.WorkOrderSummary, error)
	FindScheduledBetween(ctx context.Context, from time.Time, to time.Time) ([]*WorkOrder, error)
}

type NotificationRepository interface {
	Save(ctx context.Context, notification *notification.Notification) (*notification.Notification, error)
}

// TransactionalStore persists the aggregates changed by a work-order use case
// within the unit of work transaction.
type TransactionalStore interface {
	SaveWorkOrder(ctx context.Context, order *WorkOrder) error
	SaveNotification(ctx context.Context, notification *notification.Notification) error
}

type UnitOfWork interface {
	Execute(ctx context.Context, operation func(TransactionalStore) error) error
}

type UserRepository interface {
	FindByAuthID(auth0ID string) (user.User, error)
}

type FileURLResolver interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
