package category_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateCategoryStoresCategoryAndAuditEventInOneUnitOfWork(t *testing.T) {
	service, unit, finder, store := newCategoryServiceForTest()
	expected := &category.Category{ID: 17, Name: "Plomería", NormalizedName: "plomería"}
	finder.On("FindOperatorIDByAuthID", mock.Anything, "auth0|supervisor").Return(31, nil).Once()
	expectCategoryTransaction(unit, store)
	store.On("SaveCategory", mock.Anything, mock.MatchedBy(func(toSave category.Category) bool {
		return toSave.Name == expected.Name && toSave.NormalizedName == expected.NormalizedName
	})).Return(expected, nil).Once()
	store.On("SaveAuditEvent", mock.Anything, mock.MatchedBy(func(event *audit.Event) bool {
		return event != nil && event.OperatorID() == 31 && event.Action() == audit.ActionCreate &&
			event.ResourceType() == "category" && event.ResourceID() == "17" &&
			event.Result() == audit.ResultSucceeded && event.CorrelationID() == "request-123" &&
			event.OccurredOn().Equal(time.Date(2026, 8, 15, 17, 0, 0, 0, time.UTC)) &&
			event.OccurredOn().Location() == time.UTC
	})).Return(nil).Once()

	created, err := service.CreateCategory(context.Background(), "  Plomería  ", "auth0|supervisor", "request-123")

	require.NoError(t, err)
	assert.Equal(t, expected, created)
	finder.AssertExpectations(t)
	unit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestCreateCategoryRejectsInvalidNameBeforeOperatorLookupOrTransaction(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		expected error
	}{
		{"   ", category.ErrNameRequired},
		{strings.Repeat("a", 101), category.ErrNameTooLong},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service, unit, finder, _ := newCategoryServiceForTest()
			created, err := service.CreateCategory(context.Background(), testCase.name, "auth0|supervisor", "request-123")
			assert.ErrorIs(t, err, testCase.expected)
			assert.Nil(t, created)
			finder.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
			unit.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
		})
	}
}

func TestCreateCategoryDoesNotStartTransactionWhenOperatorCannotBeResolved(t *testing.T) {
	service, unit, finder, _ := newCategoryServiceForTest()
	operatorErr := errors.New("operator unavailable")
	finder.On("FindOperatorIDByAuthID", mock.Anything, "auth0|supervisor").Return(0, operatorErr).Once()
	created, err := service.CreateCategory(context.Background(), "Plomería", "auth0|supervisor", "request-123")
	assert.ErrorIs(t, err, operatorErr)
	assert.Nil(t, created)
	finder.AssertExpectations(t)
	unit.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestCreateCategoryPropagatesDuplicateErrorWithoutSavingAudit(t *testing.T) {
	service, unit, finder, store := newCategoryServiceForTest()
	finder.On("FindOperatorIDByAuthID", mock.Anything, "auth0|supervisor").Return(31, nil).Once()
	expectCategoryTransaction(unit, store)
	store.On("SaveCategory", mock.Anything, mock.Anything).Return((*category.Category)(nil), category.ErrAlreadyExists).Once()
	created, err := service.CreateCategory(context.Background(), "PLOMERÍA", "auth0|supervisor", "request-123")
	assert.ErrorIs(t, err, category.ErrAlreadyExists)
	assert.Nil(t, created)
	store.AssertNotCalled(t, "SaveAuditEvent", mock.Anything, mock.Anything)
	finder.AssertExpectations(t)
	unit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestCreateCategoryWrapsStorageErrorWithoutSavingAudit(t *testing.T) {
	service, unit, finder, store := newCategoryServiceForTest()
	storageErr := errors.New("category storage unavailable")
	finder.On("FindOperatorIDByAuthID", mock.Anything, "auth0|supervisor").Return(31, nil).Once()
	expectCategoryTransaction(unit, store)
	store.On("SaveCategory", mock.Anything, mock.Anything).Return((*category.Category)(nil), storageErr).Once()
	created, err := service.CreateCategory(context.Background(), "Plomería", "auth0|supervisor", "request-123")
	assert.ErrorIs(t, err, storageErr)
	assert.Nil(t, created)
	store.AssertNotCalled(t, "SaveAuditEvent", mock.Anything, mock.Anything)
	finder.AssertExpectations(t)
	unit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestCreateCategoryReturnsAuditErrorFromUnitOfWork(t *testing.T) {
	service, unit, finder, store := newCategoryServiceForTest()
	auditErr := errors.New("audit storage unavailable")
	finder.On("FindOperatorIDByAuthID", mock.Anything, "auth0|supervisor").Return(31, nil).Once()
	expectCategoryTransaction(unit, store)
	store.On("SaveCategory", mock.Anything, mock.Anything).Return(&category.Category{ID: 17}, nil).Once()
	store.On("SaveAuditEvent", mock.Anything, mock.Anything).Return(auditErr).Once()
	created, err := service.CreateCategory(context.Background(), "Plomería", "auth0|supervisor", "request-123")
	assert.ErrorIs(t, err, auditErr)
	assert.Nil(t, created)
	finder.AssertExpectations(t)
	unit.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestListCategoriesReturnsRepositoryCategories(t *testing.T) {
	expected := []category.Category{
		{ID: 1, Name: "Electricidad", NormalizedName: "electricidad"},
		{ID: 2, Name: "Plomería", NormalizedName: "plomería"},
	}
	repository := new(categoryRepositoryMock)
	repository.On("ListAll").Return(expected, nil).Once()
	service := category.NewService(repository, nil, nil, nil)
	categories, err := service.ListCategories()
	require.NoError(t, err)
	assert.Equal(t, expected, categories)
	repository.AssertExpectations(t)
}

func TestListCategoriesReturnsEmptyCollection(t *testing.T) {
	repository := new(categoryRepositoryMock)
	repository.On("ListAll").Return([]category.Category{}, nil).Once()
	service := category.NewService(repository, nil, nil, nil)
	categories, err := service.ListCategories()
	require.NoError(t, err)
	assert.Empty(t, categories)
	assert.NotNil(t, categories)
	repository.AssertExpectations(t)
}

func TestListCategoriesWrapsRepositoryError(t *testing.T) {
	repositoryError := errors.New("repository unavailable")
	repository := new(categoryRepositoryMock)
	repository.On("ListAll").Return(([]category.Category)(nil), repositoryError).Once()
	service := category.NewService(repository, nil, nil, nil)
	categories, err := service.ListCategories()
	assert.ErrorIs(t, err, repositoryError)
	assert.Nil(t, categories)
	repository.AssertExpectations(t)
}
