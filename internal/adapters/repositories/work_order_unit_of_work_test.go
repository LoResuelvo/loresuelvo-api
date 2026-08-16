package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workOrderUnitOfWorkTestContext struct {
	testContext serviceProposalRepositoryTestContext
	unit        *repositories.WorkOrderUnitOfWork
	order       *workorder.WorkOrder
	consumerID  int
	providerID  int
}

func newWorkOrderUnitOfWorkTest(t *testing.T) workOrderUnitOfWorkTestContext {
	t.Helper()
	testContext := newServiceProposalRepositoryTest(t)
	t.Cleanup(func() {
		_, _ = testContext.database.Exec("DELETE FROM notifications")
	})

	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		userRepository: testContext.userRepository,
	}, "auth0|unit-consumer", "unit.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|unit-provider", "unit.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)

	order := saveScheduledWorkOrderAt(
		t,
		testContext,
		activeConversation,
		consumerID,
		providerID,
		time.Now().UTC().Truncate(time.Microsecond).Add(48*time.Hour),
	)
	notificationRepository := repositories.NewNotificationRepository(testContext.database)
	return workOrderUnitOfWorkTestContext{
		testContext: testContext,
		unit: repositories.NewWorkOrderUnitOfWork(
			testContext.database,
			testContext.workOrderRepository,
			notificationRepository,
		),
		order:      order,
		consumerID: consumerID,
		providerID: providerID,
	}
}

func saveCompletionImageForUnitOfWorkTest(t *testing.T, testContext workOrderUnitOfWorkTestContext, fileID string) {
	t.Helper()
	_, err := testContext.testContext.database.Exec(
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, 'private', 'trabajo.jpg', 'image/jpeg', 1024, 'confirmed', 'private', 'work_order_completion_image', 'auth0|unit-provider', NOW(), NOW())`,
		fileID,
		"files/2026/08/work_order_completion_image/"+fileID+"/trabajo.jpg",
	)
	require.NoError(t, err)
}

func workOrderCompletionNotification(userID, workOrderID int) *notification.Notification {
	return &notification.Notification{
		UserID:       userID,
		Type:         notification.TypeServiceProposalAccepted,
		ResourceType: notification.ResourceWorkOrder,
		ResourceID:   workOrderID,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestWorkOrderUnitOfWorkCommitsOrderAndNotificationTogether(t *testing.T) {
	testContext := newWorkOrderUnitOfWorkTest(t)
	imageID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	saveCompletionImageForUnitOfWorkTest(t, testContext, imageID)
	report, err := workorder.NewCompletionReport(
		"Trabajo terminado y verificado.",
		[]string{imageID},
		testContext.order.ScheduledOn(),
	)
	require.NoError(t, err)
	require.NoError(t, testContext.order.ReportCompletion(testContext.providerID, report))
	event := workOrderCompletionNotification(testContext.consumerID, testContext.order.ID())

	err = testContext.unit.Execute(t.Context(), func(store workorder.TransactionalStore) error {
		if err := store.SaveWorkOrder(t.Context(), testContext.order); err != nil {
			return err
		}
		return store.SaveNotification(t.Context(), event)
	})

	require.NoError(t, err)
	foundOrder, err := testContext.testContext.workOrderRepository.FindByID(t.Context(), testContext.order.ID())
	require.NoError(t, err)
	assert.Equal(t, workorder.StatusAwaitingPayment, foundOrder.Status())
	assert.Equal(t, []string{imageID}, foundOrder.CompletionReport().ImageFileIDs())
	var notificationCount int
	require.NoError(t, testContext.testContext.database.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE resource_type = $1 AND resource_id = $2`,
		notification.ResourceWorkOrder,
		testContext.order.ID(),
	).Scan(&notificationCount))
	assert.Equal(t, 1, notificationCount)
}

func TestWorkOrderUnitOfWorkRollsBackOrderWhenNotificationFails(t *testing.T) {
	testContext := newWorkOrderUnitOfWorkTest(t)
	imageID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	saveCompletionImageForUnitOfWorkTest(t, testContext, imageID)
	report, err := workorder.NewCompletionReport(
		"Trabajo terminado y verificado.",
		[]string{imageID},
		testContext.order.ScheduledOn(),
	)
	require.NoError(t, err)
	require.NoError(t, testContext.order.ReportCompletion(testContext.providerID, report))
	event := workOrderCompletionNotification(999999, testContext.order.ID())

	err = testContext.unit.Execute(t.Context(), func(store workorder.TransactionalStore) error {
		if err := store.SaveWorkOrder(t.Context(), testContext.order); err != nil {
			return err
		}
		return store.SaveNotification(t.Context(), event)
	})

	require.Error(t, err)
	foundOrder, err := testContext.testContext.workOrderRepository.FindByID(t.Context(), testContext.order.ID())
	require.NoError(t, err)
	assert.Equal(t, workorder.StatusScheduled, foundOrder.Status())
	assert.Nil(t, foundOrder.CompletionReport())
	var notificationCount int
	require.NoError(t, testContext.testContext.database.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE resource_id = $1`,
		testContext.order.ID(),
	).Scan(&notificationCount))
	assert.Zero(t, notificationCount)
}

func TestWorkOrderUnitOfWorkRollsBackNotificationWhenOrderFails(t *testing.T) {
	testContext := newWorkOrderUnitOfWorkTest(t)
	report, err := workorder.NewCompletionReport(
		"Trabajo terminado y verificado.",
		[]string{"dddddddd-dddd-dddd-dddd-dddddddddddd"},
		testContext.order.ScheduledOn(),
	)
	require.NoError(t, err)
	require.NoError(t, testContext.order.ReportCompletion(testContext.providerID, report))
	event := workOrderCompletionNotification(testContext.consumerID, testContext.order.ID())

	err = testContext.unit.Execute(t.Context(), func(store workorder.TransactionalStore) error {
		if err := store.SaveNotification(t.Context(), event); err != nil {
			return err
		}
		return store.SaveWorkOrder(t.Context(), testContext.order)
	})

	require.ErrorIs(t, err, workorder.ErrCompletionReportImageNotAvailable)
	foundOrder, err := testContext.testContext.workOrderRepository.FindByID(t.Context(), testContext.order.ID())
	require.NoError(t, err)
	assert.Equal(t, workorder.StatusScheduled, foundOrder.Status())
	assert.Nil(t, foundOrder.CompletionReport())
	var notificationCount int
	require.NoError(t, testContext.testContext.database.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE resource_id = $1`,
		testContext.order.ID(),
	).Scan(&notificationCount))
	assert.Zero(t, notificationCount)
}

func TestWorkOrderUnitOfWorkRejectsMissingOperation(t *testing.T) {
	databaseContext := newServiceProposalRepositoryTest(t)
	unit := repositories.NewWorkOrderUnitOfWork(
		databaseContext.database,
		databaseContext.workOrderRepository,
		repositories.NewNotificationRepository(databaseContext.database),
	)

	err := unit.Execute(t.Context(), nil)

	assert.EqualError(t, err, "executing work order unit of work: operation is required")
}
