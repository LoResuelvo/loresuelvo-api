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

func (m *readerMock) FindByID(ctx context.Context, id int) (*workorder.WorkOrder, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*workorder.WorkOrder), args.Error(1)
}

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

type userRepositoryMock struct{ mock.Mock }

func (m *userRepositoryMock) FindByAuthID(auth0ID string) (user.User, error) {
	args := m.Called(auth0ID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(user.User), args.Error(1)
}

type workOrderServiceTestEnv struct {
	reader      *readerMock
	users       *userRepositoryMock
	repository  *notificationRepositoryMock
	notificator *notificatorMock
	clock       *clockMock
	service     *workorder.Service
}

func setupWorkOrderServiceTest(now time.Time) *workOrderServiceTestEnv {
	reader := new(readerMock)
	users := new(userRepositoryMock)
	repository := new(notificationRepositoryMock)
	notificator := new(notificatorMock)
	clock := new(clockMock)
	clock.On("Now").Return(now)

	return &workOrderServiceTestEnv{
		reader:      reader,
		users:       users,
		repository:  repository,
		notificator: notificator,
		clock:       clock,
		service:     workorder.NewService(reader, users, nil, repository, notificator, clock),
	}
}

func workOrderFixture(id, consumerID, providerID int, scheduledOn time.Time) *workorder.WorkOrder {
	order, err := workorder.New(&serviceproposal.ServiceProposal{
		ID:          id + 100,
		Consumer:    &consumer.Consumer{BaseUser: user.RehydrateBaseUser(consumerID, "", "", "", "", "", nil)},
		Provider:    &provider.Provider{BaseUser: user.RehydrateBaseUser(providerID, "", "", "", "", "", nil)},
		ScheduledOn: scheduledOn,
	}, time.Time{})
	if err != nil {
		panic(err)
	}
	order.SetID(id)
	return order
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
