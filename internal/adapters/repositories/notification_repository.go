package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
)

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (repository *NotificationRepository) Save(notification *notification.Notification) (*notification.Notification, error) {
	// Implement the logic to save the notification to the database
	// For example, you can use an INSERT statement to insert the notification into a notifications table
	// After saving, you can return the saved notification and any error that occurred during the process

	return notification, nil // Replace with actual implementation
}
