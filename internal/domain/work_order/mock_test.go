package workorder_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
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

func (m *notificationRepositoryMock) SaveIfAbsent(
	ctx context.Context,
	created *notification.Notification,
) (*notification.Notification, bool, error) {
	args := m.Called(ctx, created)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*notification.Notification), args.Bool(1), args.Error(2)
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

type reviewReportActorFinderMock struct{ mock.Mock }

func (m *reviewReportActorFinderMock) FindByAuthID(ctx context.Context, authID string) (int, string, error) {
	a := m.Called(ctx, authID)
	return a.Int(0), a.String(1), a.Error(2)
}

type reviewUnitOfWorkMock struct {
	mock.Mock
	store workorder.ReviewStore
}

func (m *reviewUnitOfWorkMock) Execute(ctx context.Context, op func(workorder.ReviewStore) error) error {
	a := m.Called(ctx, op)
	if err := a.Error(0); err != nil {
		return err
	}
	return op(m.store)
}

type reviewStoreMock struct{ mock.Mock }

func (m *reviewStoreMock) FindReview(ctx context.Context, id int) (*workorder.Review, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*workorder.Review), a.Error(1)
}
func (m *reviewStoreMock) FindReport(ctx context.Context, id int) (*workorder.ReviewReport, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*workorder.ReviewReport), a.Error(1)
}
func (m *reviewStoreMock) SaveReview(ctx context.Context, id int, r *workorder.Review) error {
	return m.Called(ctx, id, r).Error(0)
}
func (m *reviewStoreMock) SaveReport(ctx context.Context, r *workorder.ReviewReport) error {
	return m.Called(ctx, r).Error(0)
}
func (m *reviewStoreMock) SaveDecision(ctx context.Context, d *workorder.ReviewDecision) error {
	return m.Called(ctx, d).Error(0)
}
func (m *reviewStoreMock) SaveAuditEvent(ctx context.Context, e *audit.Event) error {
	return m.Called(ctx, e).Error(0)
}

type reviewOperatorFinderMock struct{ mock.Mock }

func (m *reviewOperatorFinderMock) FindOperatorIDByAuthID(ctx context.Context, auth string) (int, error) {
	a := m.Called(ctx, auth)
	return a.Int(0), a.Error(1)
}

type reviewAuditWriterMock struct{ mock.Mock }

func (m *reviewAuditWriterMock) Save(ctx context.Context, event *audit.Event) error {
	return m.Called(ctx, event).Error(0)
}

type adminReviewReaderMock struct{ mock.Mock }

func (m *adminReviewReaderMock) FindPage(ctx context.Context, input workorder.ReviewListInput) (*workorder.AdminReviewPage, error) {
	a := m.Called(ctx, input)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*workorder.AdminReviewPage), a.Error(1)
}
func (m *adminReviewReaderMock) FindByID(ctx context.Context, id int, page workorder.ReviewPageInput) (*workorder.AdminReviewDetail, error) {
	a := m.Called(ctx, id, page)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*workorder.AdminReviewDetail), a.Error(1)
}
