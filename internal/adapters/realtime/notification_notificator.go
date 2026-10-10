package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type notificationRecipientFinder interface {
	FindByID(ctx context.Context, id int) (user.User, error)
}

type notificationJobRequestFinder interface {
	FindByID(id int) (*jobrequest.JobRequest, error)
}
type notificationJobRequestImageResolver interface {
	ResolveJobRequestImages(ctx context.Context, images []filedomain.Image) ([]filedomain.Image, error)
}
type NotificationNotificator struct {
	jobRequests    notificationJobRequestFinder
	images         notificationJobRequestImageResolver
	dispatcher     eventDispatcher
	userRepository notificationRecipientFinder
}

func NewNotificationNotificator(dispatcher eventDispatcher, userRepository notificationRecipientFinder, jobRequests notificationJobRequestFinder, images notificationJobRequestImageResolver) *NotificationNotificator {
	return &NotificationNotificator{
		dispatcher:     dispatcher,
		jobRequests:    jobRequests,
		images:         images,
		userRepository: userRepository,
	}
}

func (n *NotificationNotificator) Notify(ctx context.Context, notice *notification.Notification) error {
	if notice == nil {
		return fmt.Errorf("notifying realtime notification: notification is required")
	}

	recipient, err := n.userRepository.FindByID(ctx, notice.UserID)
	if err != nil {
		return fmt.Errorf("finding notification recipient: %w", err)
	}
	var request *realtimeJobRequest
	if notice.Type == notification.TypeJobRequestReceived {
		request, err = n.jobRequestPayload(ctx, notice)
		if err != nil {
			return err
		}
	}
	event, err := buildNotificationEvent(notice, request)
	if err != nil {
		return fmt.Errorf("building realtime notification event: %w", err)
	}

	if err := n.dispatcher.Publish(ctx, recipient.AuthID(), recipient.Role(), recipient.ID(), event); err != nil {
		return fmt.Errorf("publishing distributed realtime notification: %w", err)
	}
	return nil
}

func buildNotificationEvent(notification *notification.Notification, request *realtimeJobRequest) ([]byte, error) {
	event := realtimeNotificationEvent{
		JobRequest: request,
		Type:       "notification.created",
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
	JobRequest   *realtimeJobRequest       `json:"job_request,omitempty"`
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

type realtimeJobRequest struct {
	ID             int                    `json:"id"`
	ConversationID int                    `json:"conversation_id"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	Requester      realtimeJobRequester   `json:"requester"`
	Images         []realtimeMessageImage `json:"images"`
}
type realtimeJobRequester struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

func (n *NotificationNotificator) jobRequestPayload(ctx context.Context, notice *notification.Notification) (*realtimeJobRequest, error) {
	request, err := n.jobRequests.FindByID(notice.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("finding notification job request: %w", err)
	}
	if request.ProviderID != notice.UserID {
		return nil, fmt.Errorf("notification recipient is not assigned to job request")
	}
	requester, err := n.userRepository.FindByID(ctx, request.ConsumerID)
	if err != nil {
		return nil, fmt.Errorf("finding job request consumer: %w", err)
	}
	images, err := n.images.ResolveJobRequestImages(ctx, request.Images)
	if err != nil {
		return nil, fmt.Errorf("resolving notification job request images: %w", err)
	}
	result := &realtimeJobRequest{
		ID: request.ID, ConversationID: request.ConversationID, Title: request.Title, Description: request.Description, Status: string(request.Status),
		Requester: realtimeJobRequester{Name: requester.Name(), Surname: requester.Surname()},
		Images:    make([]realtimeMessageImage, 0, len(images)),
	}
	for _, image := range images {
		result.Images = append(result.Images, realtimeMessageImage{ID: image.FileID, OriginalName: image.OriginalName, URL: image.URL})
	}
	return result, nil
}
