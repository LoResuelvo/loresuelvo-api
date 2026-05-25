package provider_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type providerRepositoryMock struct {
	savedProvider      provider.Provider
	saveCalled         bool
	existsByEmailValue bool
	findByEmailCalled  bool
}

type categoryRepositoryMock struct {
	categories []category.Category
}

func existingCategory() category.Category {
	category, _ := category.New("Plomería")
	category.ID = 1
	return *category
}

func categoryRepositoryWithExistingCategory() *categoryRepositoryMock {
	return &categoryRepositoryMock{categories: []category.Category{existingCategory()}}
}

func (repository *categoryRepositoryMock) FindByID(id int) *category.Category {
	for i := range repository.categories {
		if repository.categories[i].ID == id {
			return &repository.categories[i]
		}
	}
	return nil
}

func (repository *categoryRepositoryMock) FindByNormalizedName(normalizedName string) *category.Category {
	for i := range repository.categories {
		if repository.categories[i].NormalizedName == normalizedName {
			return &repository.categories[i]
		}
	}
	return nil
}

func (repository *categoryRepositoryMock) Save(categoryToSave category.Category) (*category.Category, error) {
	repository.categories = append(repository.categories, categoryToSave)
	return &repository.categories[len(repository.categories)-1], nil
}

func (repository *providerRepositoryMock) Save(provider provider.Provider) error {
	repository.savedProvider = provider
	repository.saveCalled = true
	return nil
}

func (repository *providerRepositoryMock) FindByEmail(email string) bool {
	repository.findByEmailCalled = true
	return repository.existsByEmailValue
}

func TestRegisterProviderWithValidData(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
	)

	require.NoError(t, err)
	assert.Equal(t, "auth0|ana", repository.savedProvider.User.AuthID)
	assert.Equal(t, "ana@example.com", repository.savedProvider.User.Email)
	assert.Equal(t, "Plomería", repository.savedProvider.Category.Name)
}

func TestRegisterProviderWithEmailWithoutArroba(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"anaexample.com",
		"Ana",
		"Perez",
		1,
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithEmailWithoutDomain(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@",
		"Ana",
		"Perez",
		1,
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithEmailWithoutName(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"@example.com",
		"Ana",
		"Perez",
		1,
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &providerRepositoryMock{existsByEmailValue: true}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
	)

	assert.ErrorIs(t, err, validator.ErrEmailAlreadyRegistered)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is already registered")
	assert.True(t, repository.findByEmailCalled, "email registration should be checked")
}

func TestRegisterProviderWithMissingCategory(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		0,
	)

	assert.ErrorIs(t, err, category.ErrIDRequired)
	assert.False(t, repository.saveCalled, "provider should not be saved when category is missing")
}

func TestRegisterProviderWithNonExistingCategory(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		999,
	)

	assert.ErrorIs(t, err, category.ErrDoesNotExist)
	assert.False(t, repository.saveCalled, "provider should not be saved when category does not exist")
}

func TestRegisterProviderWithWrongCategoryID(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryRepository := categoryRepositoryWithExistingCategory()
	providerManager := provider.NewService(repository, categoryRepository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		2,
	)

	assert.ErrorIs(t, err, category.ErrDoesNotExist)
	assert.False(t, repository.saveCalled, "provider should not be saved when category id does not exist")
}
