package notification

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
)

type Notification struct {
	ID           int
	UserID       int
	Type         Type
	ResourceType ResourceType
	ResourceID   int
	ReadAt       *time.Time
	CreatedAt    time.Time
}

func NewNotification(userID int, notificationType Type, resourceType ResourceType, resourceID int, clock clock.Clock) *Notification {
	return &Notification{
		UserID:       userID,
		Type:         notificationType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		CreatedAt:    clock.Now(),
	}
}
