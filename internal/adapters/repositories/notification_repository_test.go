package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationRepositoryTestContext struct {
	database               *sql.DB
	userRepository         *repositories.UserRepository
	notificationRepository *repositories.NotificationRepository
}

func newNotificationRepositoryTest(t *testing.T) notificationRepositoryTestContext {
	t.Helper()

	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanNotificationRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanNotificationRepositoryTestDatabase(t, database)

	return notificationRepositoryTestContext{
		database:               database,
		userRepository:         repositories.NewUserRepository(database),
		notificationRepository: repositories.NewNotificationRepository(database),
	}
}

func cleanNotificationRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM notifications")
	require.NoError(t, err, "could not clean notifications")

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")
}

func TestNotificationRepositoryCanSave(t *testing.T) {
	testContext := newNotificationRepositoryTest(t)
	consumerToSave := consumerWithAddress(t, testContext.database, "auth0|notification-consumer", "notification.consumer@example.com", "Ana", "Perez")
	_, err := testContext.userRepository.Save(context.Background(), consumerToSave)
	require.NoError(t, err)
	consumerID, err := testContext.userRepository.FindIDByEmail(consumerToSave.Email())
	require.NoError(t, err)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	notificationToSave := &notification.Notification{
		UserID:       consumerID,
		Type:         notification.TypeServiceProposalReceived,
		ResourceType: notification.ResourceServiceProposal,
		ResourceID:   123,
		CreatedAt:    createdAt,
	}

	savedNotification, err := testContext.notificationRepository.Save(context.Background(), notificationToSave)

	require.NoError(t, err)
	require.NotNil(t, savedNotification)
	assert.NotZero(t, savedNotification.ID)
	assert.Equal(t, consumerID, savedNotification.UserID)
	assert.Equal(t, notification.TypeServiceProposalReceived, savedNotification.Type)
	assert.Equal(t, notification.ResourceServiceProposal, savedNotification.ResourceType)
	assert.Equal(t, 123, savedNotification.ResourceID)
	assert.Nil(t, savedNotification.ReadAt)
	assert.Equal(t, createdAt, savedNotification.CreatedAt.UTC())

	var storedUserID int
	var storedType notification.Type
	var storedResourceType notification.ResourceType
	var storedResourceID int
	var storedReadAt sql.NullTime
	var storedCreatedAt time.Time
	err = testContext.database.QueryRow(
		`SELECT user_id, type, resource_type, resource_id, read_at, created_at
		FROM notifications
		WHERE id = $1`,
		savedNotification.ID,
	).Scan(&storedUserID, &storedType, &storedResourceType, &storedResourceID, &storedReadAt, &storedCreatedAt)
	require.NoError(t, err)
	assert.Equal(t, consumerID, storedUserID)
	assert.Equal(t, notification.TypeServiceProposalReceived, storedType)
	assert.Equal(t, notification.ResourceServiceProposal, storedResourceType)
	assert.Equal(t, 123, storedResourceID)
	assert.False(t, storedReadAt.Valid)
	assert.Equal(t, createdAt, storedCreatedAt.UTC())
}

func TestNotificationRepositoryFindsLatestByUserAndResource(t *testing.T) {
	testContext := newNotificationRepositoryTest(t)
	consumerToSave := consumerWithAddress(t, testContext.database, "auth0|notification-reader", "notification.reader@example.com", "Ana", "Perez")
	_, err := testContext.userRepository.Save(context.Background(), consumerToSave)
	require.NoError(t, err)
	consumerID, err := testContext.userRepository.FindIDByEmail(consumerToSave.Email())
	require.NoError(t, err)

	for index := 0; index < 2; index++ {
		_, err = testContext.notificationRepository.Save(context.Background(), &notification.Notification{
			UserID:       consumerID,
			Type:         notification.TypeWorkOrderCompletionReported,
			ResourceType: notification.ResourceWorkOrder,
			ResourceID:   42,
			CreatedAt:    time.Now().UTC().Add(time.Duration(index) * time.Second),
		})
		require.NoError(t, err)
	}

	found, err := testContext.notificationRepository.FindLatestByUserAndResource(
		context.Background(),
		consumerID,
		notification.TypeWorkOrderCompletionReported,
		notification.ResourceWorkOrder,
		42,
	)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotZero(t, found.ID)
	assert.Equal(t, consumerID, found.UserID)
	assert.Equal(t, notification.TypeWorkOrderCompletionReported, found.Type)
	assert.Equal(t, notification.ResourceWorkOrder, found.ResourceType)
	assert.Equal(t, 42, found.ResourceID)
}
