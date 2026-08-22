package notification

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
)

type Notification struct {
	ID                       int
	UserID                   int
	Type                     Type
	ResourceType             ResourceType
	ResourceID               int
	EstimatedDurationMinutes int
	ReadAt                   *time.Time
	CreatedAt                time.Time
}

func NewNotification(userID int, notificationType Type, resourceType ResourceType, resourceID int, clock clock.Clock, estimatedDuration ...int) *Notification {
	notification := &Notification{
		UserID:       userID,
		Type:         notificationType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		CreatedAt:    clock.Now(),
	}
	if len(estimatedDuration) > 0 {
		notification.EstimatedDurationMinutes = estimatedDuration[0]
	}
	return notification
}
