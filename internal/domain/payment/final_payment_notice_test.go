package payment

import (
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestFinalPaymentPersistsNoticeBeforePublishing(t *testing.T) {
	now := time.Date(2026, 7, 6, 13, 0, 0, 0, time.UTC)
	for _, failure := range []string{"none", "notification_save", "commit"} {
		t.Run(failure, func(t *testing.T) {
			order := finalPaymentOrder(t, now)
			finder := new(paymentWorkOrderFinderMock)
			finder.On("FindByServiceProposalID", mock.Anything, 42).Return(order, nil).Once()
			clock := new(adminPaymentClockMock)
			clock.On("Now").Return(now)
			store := new(paymentTransactionalStoreMock)
			store.On("SaveTransaction", mock.Anything, mock.Anything).Return(nil).Once()
			store.On("SaveIntent", mock.Anything, mock.Anything).Return(nil).Once()
			store.On("SaveWorkOrder", mock.Anything, mock.MatchedBy(func(o *workorder.WorkOrder) bool { return o.Status() == workorder.StatusPaid })).Return(nil).Once()
			var saved *notification.Notification
			var saveErr error
			if failure == "notification_save" {
				saveErr = errors.New("save failed")
			}
			store.On("SaveNotification", mock.Anything, mock.MatchedBy(func(n *notification.Notification) bool {
				return n.UserID == 20 && n.ResourceID == 84 && n.Type == notification.TypeWorkOrderFinalPaymentApproved
			})).Run(func(a mock.Arguments) { saved = a.Get(1).(*notification.Notification); saved.ID = 99 }).Return(saveErr).Once()
			notifier := new(paymentNotificatorMock)
			unit := new(paymentUnitOfWorkMock)
			var result error
			unit.On("Execute", mock.Anything, mock.Anything).Run(func(a mock.Arguments) {
				result = a.Get(1).(func(TransactionalStore) error)(store)
				if failure == "commit" {
					result = errors.New("commit failed")
				}
			}).Return(func() error { return result }).Once()
			if failure == "none" {
				notifier.On("Notify", mock.Anything, mock.MatchedBy(func(n *notification.Notification) bool { return n == saved && n.ID == 99 })).Return(nil).Once()
			}
			service := &Service{workOrderFinder: finder, clock: clock, unitOfWork: unit, notificator: notifier}
			err := (&paymentOutcomePersistence{service: service, ctx: t.Context()}).VisitServiceBalanceApproved(ServiceBalanceApproved{Intent: &Intent{ServiceProposalID: 42}, Transaction: &Transaction{}})
			if failure == "none" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				notifier.AssertNotCalled(t, "Notify", mock.Anything, mock.Anything)
			}
			finder.AssertExpectations(t)
			store.AssertExpectations(t)
			unit.AssertExpectations(t)
			notifier.AssertExpectations(t)
		})
	}
}
