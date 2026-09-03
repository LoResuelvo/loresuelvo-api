package workorder_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUrgentNotificationSavesAndNotifiesBothParticipants(t *testing.T) {
	now := time.Now()
	order := workOrderFixture(9, 10, 20, now.Add(time.Hour))
	env := setupWorkOrderServiceTest(now)
	env.reader.On("FindScheduledBetween", mock.Anything, now, now.Add(24*time.Hour)).Return([]*workorder.WorkOrder{order}, nil).Once()
	env.repository.On("SaveIfAbsent", mock.Anything, mock.MatchedBy(matchesWorkOrderNotification(10, order.ID()))).Return(&notification.Notification{ID: 1, UserID: 10}, true, nil).Once()
	env.repository.On("SaveIfAbsent", mock.Anything, mock.MatchedBy(matchesWorkOrderNotification(20, order.ID()))).Return(&notification.Notification{ID: 2, UserID: 20}, true, nil).Once()
	env.notificator.On("Notify", mock.Anything, &notification.Notification{ID: 1, UserID: 10}).Return(nil).Once()
	env.notificator.On("Notify", mock.Anything, &notification.Notification{ID: 2, UserID: 20}).Return(nil).Once()

	err := env.service.UrgentNotification(t.Context())

	require.NoError(t, err)
	env.reader.AssertExpectations(t)
	env.repository.AssertExpectations(t)
	env.notificator.AssertExpectations(t)
}

func TestUrgentNotificationDoesNothingWhenNoOrdersAreScheduled(t *testing.T) {
	now := time.Now()
	env := setupWorkOrderServiceTest(now)
	env.reader.On("FindScheduledBetween", mock.Anything, now, now.Add(24*time.Hour)).Return([]*workorder.WorkOrder{}, nil).Once()

	err := env.service.UrgentNotification(t.Context())

	require.NoError(t, err)
	env.repository.AssertNotCalled(t, "SaveIfAbsent", mock.Anything, mock.Anything)
	env.notificator.AssertNotCalled(t, "Notify", mock.Anything, mock.Anything)
}

func TestUrgentNotificationContinuesAfterFailuresAndReturnsAllErrors(t *testing.T) {
	now := time.Now()
	firstOrder := workOrderFixture(1, 10, 20, now.Add(time.Hour))
	secondOrder := workOrderFixture(2, 30, 40, now.Add(2*time.Hour))
	saveErr := errors.New("save failed")
	notifyErr := errors.New("notify failed")
	env := setupWorkOrderServiceTest(now)
	env.reader.On("FindScheduledBetween", mock.Anything, now, now.Add(24*time.Hour)).Return([]*workorder.WorkOrder{firstOrder, secondOrder}, nil).Once()
	env.repository.On("SaveIfAbsent", mock.Anything, mock.MatchedBy(notificationBelongsTo(10))).Return(nil, false, saveErr).Once()
	for _, userID := range []int{20, 30, 40} {
		saved := &notification.Notification{ID: userID, UserID: userID}
		env.repository.On("SaveIfAbsent", mock.Anything, mock.MatchedBy(notificationBelongsTo(userID))).Return(saved, true, nil).Once()
		returnedErr := error(nil)
		if userID == 20 {
			returnedErr = notifyErr
		}
		env.notificator.On("Notify", mock.Anything, saved).Return(returnedErr).Once()
	}

	err := env.service.UrgentNotification(t.Context())

	assert.ErrorIs(t, err, saveErr)
	assert.ErrorIs(t, err, notifyErr)
	env.repository.AssertNumberOfCalls(t, "SaveIfAbsent", 4)
	env.notificator.AssertNumberOfCalls(t, "Notify", 3)
}

func TestUrgentNotificationDoesNotNotifyWhenNotificationAlreadyExists(t *testing.T) {
	now := time.Now()
	order := workOrderFixture(9, 10, 20, now.Add(time.Hour))
	env := setupWorkOrderServiceTest(now)
	env.reader.On("FindScheduledBetween", mock.Anything, now, now.Add(24*time.Hour)).Return([]*workorder.WorkOrder{order}, nil).Once()
	env.repository.On("SaveIfAbsent", mock.Anything, mock.MatchedBy(matchesWorkOrderNotification(10, order.ID()))).Return(nil, false, nil).Once()
	env.repository.On("SaveIfAbsent", mock.Anything, mock.MatchedBy(matchesWorkOrderNotification(20, order.ID()))).Return(nil, false, nil).Once()

	require.NoError(t, env.service.UrgentNotification(t.Context()))
	env.notificator.AssertNotCalled(t, "Notify", mock.Anything, mock.Anything)
	env.repository.AssertExpectations(t)
}
