package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
)

type NotificationRepository struct {
	db *sql.DB
}

type notificationQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (repository *NotificationRepository) Save(ctx context.Context, notification *notification.Notification) (*notification.Notification, error) {
	return repository.save(ctx, repository.db, notification)
}

func (repository *NotificationRepository) SaveIfAbsent(
	ctx context.Context,
	notification *notification.Notification,
) (*notification.Notification, bool, error) {
	if notification == nil {
		return nil, false, fmt.Errorf("saving notification if absent: notification is required")
	}

	saved := *notification
	err := repository.db.QueryRowContext(
		ctx,
		`INSERT INTO notifications (
			user_id,
			type,
			resource_type,
			resource_id,
			read_at,
			created_at,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (user_id, type, resource_id)
		WHERE type = 'work_order_close_to_scheduled_time'
		  AND resource_type = 'work_order'
		DO NOTHING
		RETURNING id, created_at`,
		notification.UserID,
		notification.Type,
		notification.ResourceType,
		notification.ResourceID,
		notification.ReadAt,
		notification.CreatedAt,
	).Scan(&saved.ID, &saved.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("saving notification if absent: %w", err)
	}
	return &saved, true, nil
}

func (repository *NotificationRepository) FindLatestByUserAndResource(
	ctx context.Context,
	userID int,
	notificationType notification.Type,
	resourceType notification.ResourceType,
	resourceID int,
) (*notification.Notification, error) {
	var found notification.Notification
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, type, resource_type, resource_id, read_at, created_at
		FROM notifications
		WHERE user_id = $1 AND type = $2 AND resource_type = $3 AND resource_id = $4
		ORDER BY id DESC
		LIMIT 1`,
		userID,
		notificationType,
		resourceType,
		resourceID,
	).Scan(
		&found.ID,
		&found.UserID,
		&found.Type,
		&found.ResourceType,
		&found.ResourceID,
		&found.ReadAt,
		&found.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("finding latest notification: %w", err)
	}

	return &found, nil
}

func (repository *NotificationRepository) saveWithTx(
	ctx context.Context,
	tx *sql.Tx,
	notification *notification.Notification,
) (*notification.Notification, error) {
	return repository.save(ctx, tx, notification)
}

func (repository *NotificationRepository) save(
	ctx context.Context,
	queryRower notificationQueryRower,
	notification *notification.Notification,
) (*notification.Notification, error) {
	if notification == nil {
		return nil, fmt.Errorf("saving notification: notification is required")
	}

	saved := *notification
	err := queryRower.QueryRowContext(
		ctx,
		`INSERT INTO notifications (
			user_id,
			type,
			resource_type,
			resource_id,
			read_at,
			created_at,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at`,
		notification.UserID,
		notification.Type,
		notification.ResourceType,
		notification.ResourceID,
		notification.ReadAt,
		notification.CreatedAt,
	).Scan(&saved.ID, &saved.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("saving notification: %w", err)
	}

	return &saved, nil
}

func (repository *NotificationRepository) DeleteAll() error {
	_, err := repository.db.Exec("DELETE FROM notifications")
	if err != nil {
		return fmt.Errorf("deleting all notifications: %w", err)
	}
	return nil
}
