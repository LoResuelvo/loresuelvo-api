package jobrequest

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
)

type NotificationStore interface {
	Save(ctx context.Context, createdNotification *notification.Notification) (*notification.Notification, error)
}
