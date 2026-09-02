package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type notificationRecipientFinder interface {
	FindByID(ctx context.Context, id int) (user.User, error)
}

type NotificationNotificator struct {
	dispatcher     eventDispatcher
	userRepository notificationRecipientFinder
}

func NewNotificationNotificator(dispatcher eventDispatcher, userRepository notificationRecipientFinder) *NotificationNotificator {
	return &NotificationNotificator{
		dispatcher:     dispatcher,
		userRepository: userRepository,
	}
}

func (n *NotificationNotificator) Notify(ctx context.Context, notification *notification.Notification) error {
	if notification == nil {
		return fmt.Errorf("notifying realtime notification: notification is required")
	}

	recipient, err := n.userRepository.FindByID(ctx, notification.UserID)
	if err != nil {
		return fmt.Errorf("finding notification recipient: %w", err)
	}
	event, err := BuildNotificationEvent(notification)
	if err != nil {
		return fmt.Errorf("building realtime notification event: %w", err)
	}

	if err := n.dispatcher.Publish(ctx, recipient.AuthID(), recipient.Role(), recipient.ID(), event); err != nil {
		return fmt.Errorf("publishing distributed realtime notification: %w", err)
	}
	return nil
}

func BuildNotificationEvent(notification *notification.Notification) ([]byte, error) {
	if notification == nil {
		return nil, fmt.Errorf("building realtime notification event: notification is required")
	}

	event := realtimeNotificationEvent{
		Type: "notification.created",
		Notification: realtimeEventNotification{
			ID:                       notification.ID,
			UserID:                   notification.UserID,
			Type:                     string(notification.Type),
			ResourceType:             string(notification.ResourceType),
			ResourceID:               notification.ResourceID,
			EstimatedDurationMinutes: notification.EstimatedDurationMinutes,
			ReadAt:                   notification.ReadAt,
			CreatedAt:                notification.CreatedAt,
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("realtime notificator: failed to marshal notification event", "error", err)
		return nil, err
	}
	return payload, nil
}

type realtimeNotificationEvent struct {
	Type         string                    `json:"type"`
	Notification realtimeEventNotification `json:"notification"`
}

type realtimeEventNotification struct {
	ID                       int        `json:"id"`
	UserID                   int        `json:"user_id"`
	Type                     string     `json:"type"`
	ResourceType             string     `json:"resource_type"`
	ResourceID               int        `json:"resource_id"`
	EstimatedDurationMinutes int        `json:"estimated_duration_minutes,omitempty"`
	ReadAt                   *time.Time `json:"read_at"`
	CreatedAt                time.Time  `json:"created_at"`
}
