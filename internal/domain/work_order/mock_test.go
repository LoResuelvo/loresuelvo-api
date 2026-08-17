package workorder_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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

type fileServiceMock struct{ mock.Mock }

func (m *fileServiceMock) ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error) {
	args := m.Called(ctx, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *fileServiceMock) PrepareWorkOrderCompletionImages(
	ctx context.Context,
	auth0ID string,
	fileIDs []string,
) ([]filedomain.Image, error) {
	args := m.Called(ctx, auth0ID, fileIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]filedomain.Image), args.Error(1)
}

func (m *fileServiceMock) ResolveWorkOrderCompletionImages(
	ctx context.Context,
	images []filedomain.Image,
) ([]filedomain.Image, error) {
	args := m.Called(ctx, images)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]filedomain.Image), args.Error(1)
}

type transactionalStoreMock struct{ mock.Mock }

func (m *transactionalStoreMock) SaveWorkOrder(ctx context.Context, order *workorder.WorkOrder) error {
	return m.Called(ctx, order).Error(0)
}

func (m *transactionalStoreMock) SaveNotification(ctx context.Context, created *notification.Notification) error {
	return m.Called(ctx, created).Error(0)
}

type unitOfWorkMock struct {
	mock.Mock
	store workorder.TransactionalStore
}

func (m *unitOfWorkMock) Execute(ctx context.Context, operation func(workorder.TransactionalStore) error) error {
	args := m.Called(ctx, operation)
	if err := args.Error(0); err != nil {
		return err
	}
	return operation(m.store)
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
		service:     workorder.NewService(reader, users, nil, repository, notificator, nil, clock),
	}
}

func setupCompletionServiceTest(
	now time.Time,
	order *workorder.WorkOrder,
	actor user.User,
	fileService *fileServiceMock,
	unitOfWork workorder.UnitOfWork,
	notificator *notificatorMock,
) *workorder.Service {
	reader := new(readerMock)
	reader.On("FindByID", mock.Anything, order.ID()).Return(order, nil).Once()
	users := new(userRepositoryMock)
	users.On("FindByAuthID", actor.AuthID()).Return(actor, nil).Once()
	clock := new(clockMock)
	clock.On("Now").Return(now)
	return workorder.NewService(reader, users, fileService, nil, notificator, unitOfWork, clock)
}

type reviewServiceTestEnv struct {
	reader  *readerMock
	users   *userRepositoryMock
	service *workorder.Service
}

func setupReviewServiceTest(
	order *workorder.WorkOrder,
	actor user.User,
	unitOfWork workorder.UnitOfWork,
) *reviewServiceTestEnv {
	reader := new(readerMock)
	reader.On("FindByID", mock.Anything, order.ID()).Return(order, nil).Once()
	users := new(userRepositoryMock)
	users.On("FindByAuthID", actor.AuthID()).Return(actor, nil).Once()
	return &reviewServiceTestEnv{
		reader:  reader,
		users:   users,
		service: workorder.NewService(reader, users, nil, nil, nil, unitOfWork, nil),
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
