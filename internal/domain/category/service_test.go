package category_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type categoryRepositoryMock struct {
	savedCategory                   category.Category
	saveCalled                      bool
	existsByNormalizedNameValue     bool
	findByNormalizedNameCalled      bool
	requestedCategoryNormalizedName string
}

func (repository *categoryRepositoryMock) Save(categoryToSave category.Category) (*category.Category, error) {
	categoryToSave.ID = 1
	repository.savedCategory = categoryToSave
	repository.saveCalled = true
	return &repository.savedCategory, nil
}

func (repository *categoryRepositoryMock) FindByNormalizedName(normalizedName string) *category.Category {
	repository.findByNormalizedNameCalled = true
	repository.requestedCategoryNormalizedName = normalizedName
	if repository.existsByNormalizedNameValue {
		return &repository.savedCategory
	}
	return nil
}

func (repository *categoryRepositoryMock) FindByID(_ int) *category.Category {
	return nil
}

func TestCreateCategoryWithValidName(t *testing.T) {
	repository := &categoryRepositoryMock{}
	categoryManager := category.NewService(repository)

	createdCategory, err := categoryManager.CreateCategory("Plomería")

	require.NoError(t, err)
	require.NotNil(t, createdCategory)
	assert.Equal(t, 1, createdCategory.ID)
	assert.Equal(t, "Plomería", createdCategory.Name)
	assert.Equal(t, "plomería", createdCategory.NormalizedName)
	assert.True(t, repository.saveCalled, "category should be saved")
	assert.Equal(t, "Plomería", repository.savedCategory.Name)
	assert.Equal(t, "plomería", repository.savedCategory.NormalizedName)
	assert.True(t, repository.findByNormalizedNameCalled, "category existence should be checked")
	assert.Equal(t, "plomería", repository.requestedCategoryNormalizedName)
}

func TestCreateCategoryWithEmptyName(t *testing.T) {
	repository := &categoryRepositoryMock{}
	categoryManager := category.NewService(repository)

	createdCategory, err := categoryManager.CreateCategory("   ")

	assert.ErrorIs(t, err, category.ErrNameRequired)
	assert.Nil(t, createdCategory)
	assert.False(t, repository.saveCalled, "category should not be saved when name is empty")
	assert.False(t, repository.findByNormalizedNameCalled, "empty category name should not be searched")
}

func TestCreateCategoryWithAlreadyExistingName(t *testing.T) {
	repository := &categoryRepositoryMock{existsByNormalizedNameValue: true}
	categoryManager := category.NewService(repository)

	createdCategory, err := categoryManager.CreateCategory("  PLOMERÍA  ")

	assert.ErrorIs(t, err, category.ErrAlreadyExists)
	assert.Nil(t, createdCategory)
	assert.False(t, repository.saveCalled, "category should not be saved when name already exists")
	assert.True(t, repository.findByNormalizedNameCalled, "category existence should be checked")
	assert.Equal(t, "plomería", repository.requestedCategoryNormalizedName)
}
