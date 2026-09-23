package category_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/mock"
)

type fixedCategoryClock struct{ at time.Time }

func (clock fixedCategoryClock) Now() time.Time { return clock.at }

type operatorIDFinderMock struct{ mock.Mock }

func (finder *operatorIDFinderMock) FindOperatorIDByAuthID(ctx context.Context, authID string) (int, error) {
	arguments := finder.Called(ctx, authID)
	return arguments.Int(0), arguments.Error(1)
}

type categoryTransactionalStoreMock struct{ mock.Mock }

func (store *categoryTransactionalStoreMock) SaveCategory(ctx context.Context, toSave category.Category) (*category.Category, error) {
	arguments := store.Called(ctx, toSave)
	saved, _ := arguments.Get(0).(*category.Category)
	return saved, arguments.Error(1)
}

func (store *categoryTransactionalStoreMock) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	return store.Called(ctx, event).Error(0)
}

type categoryUnitOfWorkMock struct{ mock.Mock }

func (unit *categoryUnitOfWorkMock) Execute(ctx context.Context, operation func(category.TransactionalStore) error) error {
	arguments := unit.Called(ctx, operation)
	if runOperation, ok := arguments.Get(0).(func(context.Context, func(category.TransactionalStore) error) error); ok {
		return runOperation(ctx, operation)
	}
	return arguments.Error(0)
}

func newCategoryServiceForTest() (*category.Service, *categoryUnitOfWorkMock, *operatorIDFinderMock, *categoryTransactionalStoreMock) {
	unit := new(categoryUnitOfWorkMock)
	finder := new(operatorIDFinderMock)
	store := new(categoryTransactionalStoreMock)
	service := category.NewService(new(categoryRepositoryMock), unit, finder,
		fixedCategoryClock{at: time.Date(2026, 8, 15, 14, 0, 0, 0, time.FixedZone("ART", -3*60*60))})
	return service, unit, finder, store
}

func expectCategoryTransaction(unit *categoryUnitOfWorkMock, store *categoryTransactionalStoreMock) {
	unit.On("Execute", mock.Anything, mock.Anything).
		Return(func(_ context.Context, operation func(category.TransactionalStore) error) error {
			return operation(store)
		}).Once()
}

type categoryRepositoryMock struct {
	mock.Mock
}

func (repository *categoryRepositoryMock) Save(categoryToSave category.Category) (*category.Category, error) {
	arguments := repository.Called(categoryToSave)
	savedCategory, _ := arguments.Get(0).(*category.Category)
	return savedCategory, arguments.Error(1)
}

func (repository *categoryRepositoryMock) ListAll() ([]category.Category, error) {
	arguments := repository.Called()
	categories, _ := arguments.Get(0).([]category.Category)
	return categories, arguments.Error(1)
}

func (repository *categoryRepositoryMock) FindByID(id int) *category.Category {
	arguments := repository.Called(id)
	foundCategory, _ := arguments.Get(0).(*category.Category)
	return foundCategory
}
