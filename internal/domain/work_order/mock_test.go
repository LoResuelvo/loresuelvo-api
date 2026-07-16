package workorder_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
	"github.com/stretchr/testify/mock"
)

type readerMock struct{ mock.Mock }

func (m *readerMock) FindByUserID(ctx context.Context, userID int, role string) ([]readmodel.WorkOrderSummary, error) {
	args := m.Called(ctx, userID, role)
	return args.Get(0).([]readmodel.WorkOrderSummary), args.Error(1)
}

func (m *readerMock) FindScheduledBetween(ctx context.Context, from time.Time, to time.Time) ([]*workorder.WorkOrder, error) {
	args := m.Called(ctx, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*workorder.WorkOrder), args.Error(1)
}

type notificationRepositoryMock struct{ mock.Mock }

func (m *notificationRepositoryMock) Save(ctx context.Context, created *notification.Notification) (*notification.Notification, error) {
	args := m.Called(ctx, created)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*notification.Notification), args.Error(1)
}

type notificatorMock struct{ mock.Mock }

func (m *notificatorMock) Notify(ctx context.Context, saved *notification.Notification) error {
	return m.Called(ctx, saved).Error(0)
}

type clockMock struct{ mock.Mock }

func (m *clockMock) Now() time.Time { return m.Called().Get(0).(time.Time) }

type workOrderServiceTestEnv struct {
	reader      *readerMock
	repository  *notificationRepositoryMock
	notificator *notificatorMock
	clock       *clockMock
	service     *workorder.Service
}

func setupWorkOrderServiceTest(now time.Time) *workOrderServiceTestEnv {
	reader := new(readerMock)
	repository := new(notificationRepositoryMock)
	notificator := new(notificatorMock)
	clock := new(clockMock)
	clock.On("Now").Return(now)

	return &workOrderServiceTestEnv{
		reader:      reader,
		repository:  repository,
		notificator: notificator,
		clock:       clock,
		service:     workorder.NewService(reader, nil, nil, repository, notificator, clock),
	}
}

func workOrderFixture(id, consumerID, providerID int, scheduledOn time.Time) *workorder.WorkOrder {
	return &workorder.WorkOrder{
		ID: id,
		ServiceProposal: &serviceproposal.ServiceProposal{
			ID:          id + 100,
			Consumer:    &consumer.Consumer{BaseUser: user.RehydrateBaseUser(consumerID, "", "", "", "", "", nil)},
			Provider:    &provider.Provider{BaseUser: user.RehydrateBaseUser(providerID, "", "", "", "", "", nil)},
			ScheduledOn: scheduledOn,
		},
		Status: workorder.StatusScheduled,
	}
}

func matchesWorkOrderNotification(userID, workOrderID int) func(*notification.Notification) bool {
	return func(created *notification.Notification) bool {
		return created.UserID == userID &&
			created.Type == notification.TypeWorkOrderCloseToScheduledTime &&
			created.ResourceType == notification.ResourceWorkOrder &&
			created.ResourceID == workOrderID
	}
}

func notificationBelongsTo(userID int) func(*notification.Notification) bool {
	return func(created *notification.Notification) bool {
		return created.UserID == userID
	}
}
