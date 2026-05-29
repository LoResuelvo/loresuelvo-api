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
	savedProvider          provider.Provider
	providersByCategoryID  map[int][]provider.Provider
	requestedCategoryID    int
	saveCalled             bool
	findByCategoryIDCalled bool
	existsByEmailValue     bool
	findByEmailCalled      bool
}

type categoryFinderMock struct {
	categories []category.Category
}

func existingCategory() category.Category {
	category, _ := category.New("Plomería")
	category.ID = 1
	return *category
}

func categoryFinderWithExistingCategory() *categoryFinderMock {
	return &categoryFinderMock{categories: []category.Category{existingCategory()}}
}

func (finder *categoryFinderMock) FindByID(id int) *category.Category {
	for i := range finder.categories {
		if finder.categories[i].ID == id {
			return &finder.categories[i]
		}
	}
	return nil
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

func (repository *providerRepositoryMock) FindByCategoryID(categoryID int) ([]provider.Provider, error) {
	repository.findByCategoryIDCalled = true
	repository.requestedCategoryID = categoryID
	return repository.providersByCategoryID[categoryID], nil
}

func TestRegisterProviderWithValidData(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

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

func TestFilterProvidersByCategoryID(t *testing.T) {
	providerCategory := existingCategory()
	providerToReturn, err := provider.NewProvider("auth0|ana", "ana@example.com", "Ana", "Perez", &providerCategory)
	require.NoError(t, err)
	providerToReturn.ID = 1
	repository := &providerRepositoryMock{
		providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*providerToReturn},
		},
	}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

	providers, err := providerManager.FilterProvidersByCategoryID(1)

	require.NoError(t, err)
	assert.Len(t, providers, 1)
	assert.Equal(t, 1, providers[0].ID)
	assert.Equal(t, "Ana", providers[0].User.Name)
	assert.Equal(t, "Perez", providers[0].User.Surname)
	assert.Equal(t, "Plomería", providers[0].Category.Name)
	assert.True(t, repository.findByCategoryIDCalled, "providers should be searched by category id")
	assert.Equal(t, providerCategory.ID, repository.requestedCategoryID)
}

func TestFilterProvidersByCategoryIDFindsExistingCategory(t *testing.T) {
	repository := &providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{}}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

	_, err := providerManager.FilterProvidersByCategoryID(1)

	require.NoError(t, err)
	assert.True(t, repository.findByCategoryIDCalled, "providers should be searched when category exists")
	assert.Equal(t, 1, repository.requestedCategoryID)
}

func TestFilterProvidersByCategoryIDReturnsEmptyListWhenNoProvidersExist(t *testing.T) {
	repository := &providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{}}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

	providers, err := providerManager.FilterProvidersByCategoryID(1)

	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestFilterProvidersByCategoryIDRequiresCategoryID(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

	providers, err := providerManager.FilterProvidersByCategoryID(0)

	assert.ErrorIs(t, err, category.ErrIDRequired)
	assert.Nil(t, providers)
	assert.False(t, repository.findByCategoryIDCalled, "providers should not be searched when category id is missing")
}

func TestFilterProvidersByCategoryIDRequiresExistingCategory(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder)

	providers, err := providerManager.FilterProvidersByCategoryID(999)

	assert.ErrorIs(t, err, category.ErrDoesNotExist)
	assert.Nil(t, providers)
	assert.False(t, repository.findByCategoryIDCalled, "providers should not be searched when category does not exist")
}
