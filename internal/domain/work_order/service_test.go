package workorder_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConsumerGetsDecryptedConfirmationCodeForFullyPaidWorkOrder(t *testing.T) {
	now := time.Date(2026, time.July, 6, 13, 5, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, now)
	authorization, err := workorder.NewCompletionAuthorization([]byte("encrypted-code"), now)
	require.NoError(t, err)
	require.NoError(t, order.CompletePayment(authorization))
	consumerUser := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10, "auth0|consumer", "ana@example.com", "Ana", "Pérez", consumer.Role, nil,
	)}
	env := setupWorkOrderServiceTest(now)
	env.users.On("FindByAuthID", "auth0|consumer").Return(consumerUser, nil).Once()
	env.reader.On("FindByID", mock.Anything, order.ID).Return(order, nil).Once()
	env.decryptor.On("Decrypt", []byte("encrypted-code")).Return("0042", nil).Once()

	code, err := env.service.GetConfirmationCode(t.Context(), "auth0|consumer", order.ID)

	require.NoError(t, err)
	assert.Equal(t, "0042", code.String())
	env.users.AssertExpectations(t)
	env.reader.AssertExpectations(t)
	env.decryptor.AssertExpectations(t)
}

func TestUrgentNotificationSavesAndNotifiesBothParticipants(t *testing.T) {
	now := time.Now()
	order := workOrderFixture(9, 10, 20, now.Add(time.Hour))
	env := setupWorkOrderServiceTest(now)
	env.reader.On("FindScheduledBetween", mock.Anything, now, now.Add(24*time.Hour)).Return([]*workorder.WorkOrder{order}, nil).Once()
	env.repository.On("Save", mock.Anything, mock.MatchedBy(matchesWorkOrderNotification(10, order.ID))).Return(&notification.Notification{ID: 1, UserID: 10}, nil).Once()
	env.repository.On("Save", mock.Anything, mock.MatchedBy(matchesWorkOrderNotification(20, order.ID))).Return(&notification.Notification{ID: 2, UserID: 20}, nil).Once()
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
	env.repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
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
	env.repository.On("Save", mock.Anything, mock.MatchedBy(notificationBelongsTo(10))).Return(nil, saveErr).Once()
	for _, userID := range []int{20, 30, 40} {
		saved := &notification.Notification{ID: userID, UserID: userID}
		env.repository.On("Save", mock.Anything, mock.MatchedBy(notificationBelongsTo(userID))).Return(saved, nil).Once()
		returnedErr := error(nil)
		if userID == 20 {
			returnedErr = notifyErr
		}
		env.notificator.On("Notify", mock.Anything, saved).Return(returnedErr).Once()
	}

	err := env.service.UrgentNotification(t.Context())

	assert.ErrorIs(t, err, saveErr)
	assert.ErrorIs(t, err, notifyErr)
	env.repository.AssertNumberOfCalls(t, "Save", 4)
	env.notificator.AssertNumberOfCalls(t, "Notify", 3)
}
