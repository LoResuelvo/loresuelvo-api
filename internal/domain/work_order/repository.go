package workorder

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Reader interface {
	FindByUserID(ctx context.Context, userID int, viewerRole string) ([]readmodel.WorkOrderSummary, error)
	FindWithLessScheduledTimeThan(ctx context.Context, actualTime time.Time) ([]*WorkOrder, error)
}

type NotificationRepository interface {
	Save(ctx context.Context, notification *notification.Notification) (*notification.Notification, error)
}

type UserRepository interface {
	FindByAuthID(auth0ID string) (user.User, error)
}

type FileURLResolver interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
