package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
)

type NotificationNotificator struct {
	hub            *Hub
	userRepository userAuthIDFinder
}

func NewNotificationNotificator(hub *Hub, userRepository userAuthIDFinder) *NotificationNotificator {
	return &NotificationNotificator{
		hub:            hub,
		userRepository: userRepository,
	}
}

func (n *NotificationNotificator) Notify(ctx context.Context, notification *notification.Notification) error {
	if notification == nil {
		return fmt.Errorf("notifying realtime notification: notification is required")
	}

	authID, err := n.userRepository.FindAuthIDByID(notification.UserID)
	if err != nil {
		return fmt.Errorf("finding notification user auth id: %w", err)
	}

	event, err := BuildNotificationEvent(notification)
	if err != nil {
		return fmt.Errorf("building realtime notification event: %w", err)
	}

	n.hub.BroadcastToParticipant(ctx, authID, consumer.Role, notification.UserID, event)
	return nil
}

func BuildNotificationEvent(notification *notification.Notification) ([]byte, error) {
	if notification == nil {
		return nil, fmt.Errorf("building realtime notification event: notification is required")
	}

	event := realtimeNotificationEvent{
		Type: "notification.created",
		Notification: realtimeEventNotification{
			ID:           notification.ID,
			UserID:       notification.UserID,
			Type:         string(notification.Type),
			ResourceType: string(notification.ResourceType),
			ResourceID:   notification.ResourceID,
			ReadAt:       notification.ReadAt,
			CreatedAt:    notification.CreatedAt,
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
	ID           int        `json:"id"`
	UserID       int        `json:"user_id"`
	Type         string     `json:"type"`
	ResourceType string     `json:"resource_type"`
	ResourceID   int        `json:"resource_id"`
	ReadAt       *time.Time `json:"read_at"`
	CreatedAt    time.Time  `json:"created_at"`
}
