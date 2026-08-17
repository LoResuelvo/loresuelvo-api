package workorder_test

import (
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateReviewPersistsReview(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	expectedReview := reviewForService(t)
	store := new(transactionalStoreMock)
	store.On("SaveWorkOrder", mock.Anything, mock.MatchedBy(func(saved *workorder.WorkOrder) bool {
		return saved.Review() != nil &&
			saved.Review().Rating() == expectedReview.Rating() &&
			saved.Review().Description() == expectedReview.Description()
	})).Return(nil).Once()
	unitOfWork := &unitOfWorkMock{store: store}
	unitOfWork.On("Execute", mock.Anything, mock.Anything).Return(nil).Once()
	env := setupReviewServiceTest(order, reviewer, unitOfWork)

	_, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 5, "  Trabajo prolijo  ")

	require.NoError(t, err)
	unitOfWork.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestCreateReviewReturnsReviewReadModel(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	store := new(transactionalStoreMock)
	store.On("SaveWorkOrder", mock.Anything, mock.Anything).Return(nil).Once()
	unitOfWork := &unitOfWorkMock{store: store}
	unitOfWork.On("Execute", mock.Anything, mock.Anything).Return(nil).Once()
	env := setupReviewServiceTest(order, reviewer, unitOfWork)

	result, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 5, "Trabajo prolijo")

	require.NoError(t, err)
	require.Equal(t, &readmodel.Review{Rating: 5, Description: "Trabajo prolijo"}, result)
}

func TestCreateReviewRejectsProvider(t *testing.T) {
	order, _ := paidWorkOrderForReview(t)
	actor := &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "auth0|provider", "provider@example.com", "Juan", "Provider", provider.Role, nil)}
	unitOfWork := new(unitOfWorkMock)
	env := setupReviewServiceTest(order, actor, unitOfWork)

	_, err := env.service.CreateReview(t.Context(), actor.AuthID(), order.ID(), 5, "Trabajo correcto")

	require.ErrorIs(t, err, workorder.ErrOnlyWorkOrderConsumerCanReview)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestCreateReviewReturnsNotFoundForMissingOrder(t *testing.T) {
	actor := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(10, "auth0|consumer", "consumer@example.com", "Ana", "Consumer", consumer.Role, nil)}
	reader := new(readerMock)
	reader.On("FindByID", mock.Anything, 999).Return(nil, nil).Once()
	users := new(userRepositoryMock)
	users.On("FindByAuthID", actor.AuthID()).Return(actor, nil).Once()
	service := workorder.NewService(reader, users, nil, nil, nil, nil, nil)

	_, err := service.CreateReview(t.Context(), actor.AuthID(), 999, 5, "Trabajo correcto")

	require.ErrorIs(t, err, workorder.ErrDoesNotExist)
}

func TestCreateReviewRejectsAnotherConsumer(t *testing.T) {
	order, _ := paidWorkOrderForReview(t)
	actor := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(99, "auth0|other", "other@example.com", "Other", "Consumer", consumer.Role, nil)}
	unitOfWork := new(unitOfWorkMock)
	env := setupReviewServiceTest(order, actor, unitOfWork)

	_, err := env.service.CreateReview(t.Context(), actor.AuthID(), order.ID(), 5, "Trabajo correcto")

	require.ErrorIs(t, err, workorder.ErrOnlyWorkOrderConsumerCanReview)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestCreateReviewRejectsUnpaidOrder(t *testing.T) {
	for _, status := range []workorder.Status{workorder.StatusScheduled, workorder.StatusAwaitingPayment} {
		t.Run(string(status), func(t *testing.T) {
			order, reviewer := workOrderWithStatusForReview(t, status)
			unitOfWork := new(unitOfWorkMock)
			env := setupReviewServiceTest(order, reviewer, unitOfWork)

			_, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 5, "Trabajo correcto")

			require.ErrorIs(t, err, workorder.ErrWorkOrderNotPaid)
			unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
		})
	}
}

func TestCreateReviewRejectsDuplicateReview(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	firstReview := reviewForService(t)
	require.NoError(t, order.AddReview(reviewer, firstReview))
	unitOfWork := new(unitOfWorkMock)
	env := setupReviewServiceTest(order, reviewer, unitOfWork)

	_, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 3, "Otra opinión")

	require.ErrorIs(t, err, workorder.ErrReviewAlreadyExists)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestCreateReviewReturnsReviewValidationError(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	unitOfWork := new(unitOfWorkMock)
	env := setupReviewServiceTest(order, reviewer, unitOfWork)

	_, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 0, "Trabajo correcto")

	require.ErrorIs(t, err, workorder.ErrReviewRatingOutOfRange)
	unitOfWork.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestCreateReviewRequiresUnitOfWork(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	env := setupReviewServiceTest(order, reviewer, nil)

	_, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 5, "Trabajo correcto")

	require.ErrorIs(t, err, workorder.ErrWorkOrderUnitOfWorkRequired)
}

func TestCreateReviewPropagatesPersistenceError(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	persistenceErr := errors.New("persistence failed")
	unitOfWork := new(unitOfWorkMock)
	unitOfWork.On("Execute", mock.Anything, mock.Anything).Return(persistenceErr).Once()
	env := setupReviewServiceTest(order, reviewer, unitOfWork)

	_, err := env.service.CreateReview(t.Context(), reviewer.AuthID(), order.ID(), 5, "Trabajo correcto")

	require.ErrorIs(t, err, persistenceErr)
	unitOfWork.AssertExpectations(t)
}

func TestGetWorkOrderIncludesReview(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	review := reviewForService(t)
	require.NoError(t, order.AddReview(reviewer, review))
	fileService := new(fileServiceMock)
	fileService.On("ResolveWorkOrderCompletionImages", mock.Anything, mock.Anything).Return(nil, nil).Once()
	env := setupWorkOrderDetailServiceTest(order, reviewer)
	env.service = workorder.NewService(env.reader, env.users, fileService, nil, nil, nil, nil)

	detail, err := env.service.GetWorkOrder(t.Context(), reviewer.AuthID(), order.ID())

	require.NoError(t, err)
	require.Equal(t, &readmodel.Review{Rating: 5, Description: "Trabajo prolijo"}, detail.Review)
}

func reviewForService(t *testing.T) *workorder.Review {
	t.Helper()
	review, err := workorder.NewReview(5, "  Trabajo prolijo  ")
	require.NoError(t, err)
	return review
}
