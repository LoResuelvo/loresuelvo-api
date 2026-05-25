package category_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/assert"
)

type categoryRepositoryMock struct {
	savedCategory                   category.Category
	saveCalled                      bool
	existsByNormalizedNameValue     bool
	findByNormalizedNameCalled      bool
	requestedCategoryNormalizedName string
}

func (repository *categoryRepositoryMock) Save(category category.Category) error {
	repository.savedCategory = category
	repository.saveCalled = true
	return nil
}

func (repository *categoryRepositoryMock) FindByNormalizedName(normalizedName string) bool {
	repository.findByNormalizedNameCalled = true
	repository.requestedCategoryNormalizedName = normalizedName
	return repository.existsByNormalizedNameValue
}

func TestCreateCategoryWithValidName(t *testing.T) {
	repository := &categoryRepositoryMock{}
	categoryManager := category.NewService(repository)

	err := categoryManager.CreateCategory("Plomería")

	assert.NoError(t, err)
	assert.True(t, repository.saveCalled, "category should be saved")
	assert.Equal(t, "Plomería", repository.savedCategory.Name)
	assert.Equal(t, "plomería", repository.savedCategory.NormalizedName)
	assert.True(t, repository.findByNormalizedNameCalled, "category existence should be checked")
	assert.Equal(t, "plomería", repository.requestedCategoryNormalizedName)
}

func TestCreateCategoryWithEmptyName(t *testing.T) {
	repository := &categoryRepositoryMock{}
	categoryManager := category.NewService(repository)

	err := categoryManager.CreateCategory("   ")

	assert.ErrorIs(t, err, category.ErrNameRequired)
	assert.False(t, repository.saveCalled, "category should not be saved when name is empty")
	assert.False(t, repository.findByNormalizedNameCalled, "empty category name should not be searched")
}

func TestCreateCategoryWithAlreadyExistingName(t *testing.T) {
	repository := &categoryRepositoryMock{existsByNormalizedNameValue: true}
	categoryManager := category.NewService(repository)

	err := categoryManager.CreateCategory("  PLOMERÍA  ")

	assert.ErrorIs(t, err, category.ErrAlreadyExists)
	assert.False(t, repository.saveCalled, "category should not be saved when name already exists")
	assert.True(t, repository.findByNormalizedNameCalled, "category existence should be checked")
	assert.Equal(t, "plomería", repository.requestedCategoryNormalizedName)
}
