package category_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateCategoryStoresNormalizedCategory(t *testing.T) {
	repository := new(categoryRepositoryMock)
	repository.On("Save", mock.MatchedBy(func(categoryToSave category.Category) bool {
		return categoryToSave.Name == "Plomería" && categoryToSave.NormalizedName == "plomería"
	})).Return(&category.Category{ID: 1, Name: "Plomería", NormalizedName: "plomería"}, nil).Once()
	service := category.NewService(repository)

	createdCategory, err := service.CreateCategory("  Plomería  ")

	require.NoError(t, err)
	require.NotNil(t, createdCategory)
	assert.Equal(t, 1, createdCategory.ID)
	assert.Equal(t, "Plomería", createdCategory.Name)
	assert.Equal(t, "plomería", createdCategory.NormalizedName)
	repository.AssertExpectations(t)
}

func TestCreateCategoryRejectsEmptyNameWithoutPersistence(t *testing.T) {
	repository := new(categoryRepositoryMock)
	service := category.NewService(repository)

	createdCategory, err := service.CreateCategory("   ")

	assert.ErrorIs(t, err, category.ErrNameRequired)
	assert.Nil(t, createdCategory)
	repository.AssertNotCalled(t, "Save", mock.Anything)
}

func TestCreateCategoryRejectsTooLongNameWithoutPersistence(t *testing.T) {
	repository := new(categoryRepositoryMock)
	service := category.NewService(repository)

	createdCategory, err := service.CreateCategory(strings.Repeat("a", 101))

	assert.ErrorIs(t, err, category.ErrNameTooLong)
	assert.Nil(t, createdCategory)
	repository.AssertNotCalled(t, "Save", mock.Anything)
}

func TestCreateCategoryPropagatesDuplicateError(t *testing.T) {
	repository := new(categoryRepositoryMock)
	repository.On("Save", mock.Anything).Return((*category.Category)(nil), category.ErrAlreadyExists).Once()
	service := category.NewService(repository)

	createdCategory, err := service.CreateCategory("PLOMERÍA")

	assert.ErrorIs(t, err, category.ErrAlreadyExists)
	assert.Nil(t, createdCategory)
	repository.AssertExpectations(t)
}

func TestCreateCategoryWrapsRepositoryError(t *testing.T) {
	repositoryError := errors.New("repository unavailable")
	repository := new(categoryRepositoryMock)
	repository.On("Save", mock.Anything).Return((*category.Category)(nil), repositoryError).Once()
	service := category.NewService(repository)

	createdCategory, err := service.CreateCategory("Plomería")

	assert.ErrorIs(t, err, repositoryError)
	assert.Nil(t, createdCategory)
	repository.AssertExpectations(t)
}

func TestListCategoriesReturnsRepositoryCategories(t *testing.T) {
	expected := []category.Category{
		{ID: 1, Name: "Electricidad", NormalizedName: "electricidad"},
		{ID: 2, Name: "Plomería", NormalizedName: "plomería"},
	}
	repository := new(categoryRepositoryMock)
	repository.On("ListAll").Return(expected, nil).Once()
	service := category.NewService(repository)

	categories, err := service.ListCategories()

	require.NoError(t, err)
	assert.Equal(t, expected, categories)
	repository.AssertExpectations(t)
}

func TestListCategoriesReturnsEmptyCollection(t *testing.T) {
	repository := new(categoryRepositoryMock)
	repository.On("ListAll").Return([]category.Category{}, nil).Once()
	service := category.NewService(repository)

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
	service := category.NewService(repository)

	categories, err := service.ListCategories()

	assert.ErrorIs(t, err, repositoryError)
	assert.Nil(t, categories)
	repository.AssertExpectations(t)
}
