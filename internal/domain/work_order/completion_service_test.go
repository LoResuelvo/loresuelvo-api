package workorder_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReportCompletionPersistsOrderAndNotifiesAfterCommit(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	preparedImages := []filedomain.Image{{FileID: "file-1", OriginalName: "trabajo.jpg", URL: "https://private/file-1"}}
	fileService := new(fileServiceMock)
	fileService.On("PrepareWorkOrderCompletionImages", mock.Anything, actor.AuthID(), []string{"file-1"}).Return(preparedImages, nil).Once()
	store := new(transactionalStoreMock)
	var savedOrder *workorder.WorkOrder
	store.On("SaveWorkOrder", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		savedOrder = args.Get(1).(*workorder.WorkOrder)
		savedOrder.CompletionReport().SetID(42)
	}).Return(nil).Once()
	var savedNotification *notification.Notification
	store.On("SaveNotification", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		savedNotification = args.Get(1).(*notification.Notification)
		savedNotification.ID = 84
	}).Return(nil).Once()
	unitOfWork := &unitOfWorkMock{store: store}
	unitOfWork.On("Execute", mock.Anything, mock.Anything).Return(nil).Once()
	notificator := new(notificatorMock)
	notificator.On("Notify", mock.Anything, mock.MatchedBy(func(event *notification.Notification) bool {
		return event.ID == 84 &&
			event.UserID == order.ConsumerID() &&
			event.Type == notification.TypeWorkOrderCompletionReported &&
			event.ResourceType == notification.ResourceWorkOrder &&
			event.ResourceID == order.ID()
	})).Return(errors.New("realtime unavailable")).Once()
	service := setupCompletionServiceTest(now, order, actor, fileService, unitOfWork, notificator)

	result, err := service.ReportCompletion(
		t.Context(),
		actor.AuthID(),
		order.ID(),
		"Trabajo terminado y verificado.",
		[]string{"file-1"},
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 42, result.ID)
	assert.Equal(t, "Trabajo terminado y verificado.", result.Description)
	assert.Equal(t, now, result.ReportedOn)
	assert.Equal(t, preparedImages, result.Images)
	assert.Equal(t, workorder.StatusAwaitingPayment, savedOrder.Status())
	assert.Equal(t, notification.TypeWorkOrderCompletionReported, savedNotification.Type)
	fileService.AssertExpectations(t)
	store.AssertExpectations(t)
	unitOfWork.AssertExpectations(t)
	notificator.AssertExpectations(t)
}

func TestReportCompletionRejectsNonAssignedUserBeforeValidatingFiles(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := user.RehydrateBaseUser(10, "auth0|consumer", "consumer@example.com", "Ana", "Perez", consumer.Role, nil)
	fileService := new(fileServiceMock)
	unitOfWork := &unitOfWorkMock{store: new(transactionalStoreMock)}
	service := setupCompletionServiceTest(now, order, actor, fileService, unitOfWork, nil)

	_, err := service.ReportCompletion(t.Context(), actor.AuthID(), order.ID(), "Trabajo terminado", []string{"file-1"})

	assert.ErrorIs(t, err, workorder.ErrOnlyAssignedProviderCanReport)
	fileService.AssertNotCalled(t, "PrepareWorkOrderCompletionImages", mock.Anything, mock.Anything, mock.Anything)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestReportCompletionMapsUnavailableImageError(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	fileService := new(fileServiceMock)
	fileService.On("PrepareWorkOrderCompletionImages", mock.Anything, actor.AuthID(), []string{"file-1"}).Return(nil, filedomain.ErrWorkOrderCompletionImageNotAvailable).Once()
	unitOfWork := &unitOfWorkMock{store: new(transactionalStoreMock)}
	service := setupCompletionServiceTest(now, order, actor, fileService, unitOfWork, nil)

	_, err := service.ReportCompletion(t.Context(), actor.AuthID(), order.ID(), "Trabajo terminado", []string{"file-1"})

	assert.ErrorIs(t, err, workorder.ErrWorkOrderCompletionImageNotAvailable)
	fileService.AssertExpectations(t)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestReportCompletionRejectsBeforeScheduledTime(t *testing.T) {
	now := time.Date(2026, time.August, 15, 14, 59, 59, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(time.Second))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	fileService := new(fileServiceMock)
	fileService.On("PrepareWorkOrderCompletionImages", mock.Anything, actor.AuthID(), []string{"file-1"}).Return([]filedomain.Image{{FileID: "file-1", OriginalName: "trabajo.jpg"}}, nil).Once()
	unitOfWork := &unitOfWorkMock{store: new(transactionalStoreMock)}
	service := setupCompletionServiceTest(now, order, actor, fileService, unitOfWork, nil)

	_, err := service.ReportCompletion(t.Context(), actor.AuthID(), order.ID(), "Trabajo terminado", []string{"file-1"})

	assert.ErrorIs(t, err, workorder.ErrWorkOrderNotReadyForCompletion)
	fileService.AssertExpectations(t)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestReportCompletionRequiresUnitOfWork(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	fileService := new(fileServiceMock)
	fileService.On("PrepareWorkOrderCompletionImages", mock.Anything, actor.AuthID(), []string{"file-1"}).Return([]filedomain.Image{{FileID: "file-1", OriginalName: "trabajo.jpg"}}, nil).Once()
	service := setupCompletionServiceTest(now, order, actor, fileService, nil, nil)

	_, err := service.ReportCompletion(t.Context(), actor.AuthID(), order.ID(), "Trabajo terminado", []string{"file-1"})

	assert.ErrorIs(t, err, workorder.ErrWorkOrderUnitOfWorkRequired)
	fileService.AssertExpectations(t)
}

func TestReportCompletionPropagatesPersistenceErrorAndDoesNotNotify(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	order := workOrderFixture(42, 10, 20, now.Add(-time.Hour))
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Gomez", provider.Role, nil)}
	persistenceErr := errors.New("persistence failed")
	fileService := new(fileServiceMock)
	fileService.On("PrepareWorkOrderCompletionImages", mock.Anything, actor.AuthID(), []string{"file-1"}).Return([]filedomain.Image{{FileID: "file-1", OriginalName: "trabajo.jpg"}}, nil).Once()
	unitOfWork := new(unitOfWorkMock)
	unitOfWork.On("Execute", mock.Anything, mock.Anything).Return(persistenceErr).Once()
	notificator := new(notificatorMock)
	service := setupCompletionServiceTest(now, order, actor, fileService, unitOfWork, notificator)

	_, err := service.ReportCompletion(t.Context(), actor.AuthID(), order.ID(), "Trabajo terminado", []string{"file-1"})

	assert.ErrorIs(t, err, persistenceErr)
	fileService.AssertExpectations(t)
	unitOfWork.AssertExpectations(t)
	notificator.AssertNotCalled(t, "Notify", mock.Anything, mock.Anything)
}
