package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
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
	saveErr                error
	saveID                 int
	findByCategoryIDErr    error
}

type categoryFinderMock struct {
	categories []category.Category
}

type profilePhotoValidatorMock struct {
	err                    error
	validatedAuthID        string
	validatedFileID        string
	resolvedFileIDs        []string
	profilePhotoURLsByFile map[string]string
	resolveErr             error
}

func (m *profilePhotoValidatorMock) ValidateProviderProfilePhoto(_ context.Context, authID, fileID string) error {
	m.validatedAuthID = authID
	m.validatedFileID = fileID
	return m.err
}

func (m *profilePhotoValidatorMock) ResolvePublicURL(ctx context.Context, fileID string) (string, error) {
	m.resolvedFileIDs = []string{fileID}
	urlsByFileID, err := m.ResolvePublicURLs(ctx, []string{fileID})
	if err != nil {
		return "", err
	}

	return urlsByFileID[fileID], nil
}

func (m *profilePhotoValidatorMock) ResolvePublicURLs(_ context.Context, fileIDs []string) (map[string]string, error) {
	m.resolvedFileIDs = fileIDs
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	if m.profilePhotoURLsByFile != nil {
		return m.profilePhotoURLsByFile, nil
	}
	return map[string]string{}, nil
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

func (repository *providerRepositoryMock) Save(_ context.Context, userToSave user.User) (user.User, error) {
	providerToSave := userToSave.(*provider.Provider)
	repository.savedProvider = *providerToSave
	repository.saveCalled = true
	if repository.saveErr != nil {
		return nil, repository.saveErr
	}
	if repository.saveID != 0 {
		providerToSave.ID = repository.saveID
		return providerToSave, nil
	}
	providerToSave.ID = 1
	return providerToSave, nil
}

func (repository *providerRepositoryMock) FindByEmail(email string) bool {
	repository.findByEmailCalled = true
	return repository.existsByEmailValue
}

func (repository *providerRepositoryMock) FindProvidersByCategoryID(categoryID int) ([]provider.Provider, error) {
	repository.findByCategoryIDCalled = true
	repository.requestedCategoryID = categoryID
	if repository.findByCategoryIDErr != nil {
		return nil, repository.findByCategoryIDErr
	}
	return repository.providersByCategoryID[categoryID], nil
}

func TestRegisterProviderWithValidData(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{profilePhotoURLsByFile: map[string]string{"profile-photo-file-id": "https://cdn/profile-photo.jpg"}}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	createdProvider, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"profile-photo-file-id",
	)

	require.NoError(t, err)
	assert.Equal(t, "auth0|ana", repository.savedProvider.AuthID())
	assert.Equal(t, "ana@example.com", repository.savedProvider.Email())
	assert.Equal(t, "Plomería", repository.savedProvider.Category.Name)
	require.NotNil(t, createdProvider)
	assert.Equal(t, 1, createdProvider.ID)
	assert.Equal(t, "Ana", createdProvider.Name)
	assert.Equal(t, "Perez", createdProvider.Surname)
	assert.Equal(t, "Plomería", createdProvider.CategoryName)
	assert.Equal(t, "https://cdn/profile-photo.jpg", createdProvider.ProfilePhotoURL)
}

func TestNewProviderRequiresCategory(t *testing.T) {
	createdProvider, err := provider.NewProvider("auth0|ana", "ana@example.com", "Ana", "Perez", nil, "profile-photo-file-id")

	assert.Nil(t, createdProvider)
	assert.ErrorIs(t, err, category.ErrDoesNotExist)
}

func TestNewProviderExposesUserFieldsThroughAccessors(t *testing.T) {
	providerCategory := existingCategory()
	createdProvider, err := provider.NewProvider("auth0|ana", "ana@example.com", "Ana", "Perez", &providerCategory, "profile-photo-file-id")

	require.NoError(t, err)
	assert.Equal(t, "auth0|ana", createdProvider.AuthID())
	assert.Equal(t, "ana@example.com", createdProvider.Email())
	assert.Equal(t, "Ana", createdProvider.Name())
	assert.Equal(t, "Perez", createdProvider.Surname())
	assert.Equal(t, provider.Role, createdProvider.BaseUser.Role)
}

func TestRegisterProviderWithEmailWithoutArroba(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"anaexample.com",
		"Ana",
		"Perez",
		1,
		"",
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderReturnsRepositorySaveError(t *testing.T) {
	expectedErr := errors.New("save provider")
	repository := &providerRepositoryMock{saveErr: expectedErr}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{profilePhotoURLsByFile: map[string]string{"profile-photo-file-id": "https://cdn/profile-photo.jpg"}}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	createdProvider, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"profile-photo-file-id",
	)

	assert.Nil(t, createdProvider)
	assert.ErrorIs(t, err, expectedErr)
	assert.True(t, repository.saveCalled)
}

func TestRegisterProviderReturnsProfilePhotoURLResolutionErrorBeforeSaving(t *testing.T) {
	expectedErr := errors.New("file repository unavailable")
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{resolveErr: expectedErr}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	createdProvider, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"profile-photo-file-id",
	)

	assert.Nil(t, createdProvider)
	assert.ErrorIs(t, err, expectedErr)
	assert.False(t, repository.saveCalled)
	assert.Equal(t, []string{"profile-photo-file-id"}, profilePhotoValidator.resolvedFileIDs)
}

func TestRegisterProviderWithEmailWithoutDomain(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@",
		"Ana",
		"Perez",
		1,
		"",
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithEmailWithoutName(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"@example.com",
		"Ana",
		"Perez",
		1,
		"",
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &providerRepositoryMock{existsByEmailValue: true}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"",
	)

	assert.ErrorIs(t, err, validator.ErrEmailAlreadyRegistered)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is already registered")
	assert.True(t, repository.findByEmailCalled, "email registration should be checked")
}

func TestRegisterProviderWithMissingCategory(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		0,
		"",
	)

	assert.ErrorIs(t, err, category.ErrIDRequired)
	assert.False(t, repository.saveCalled, "provider should not be saved when category is missing")
}

func TestRegisterProviderWithNonExistingCategory(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		999,
		"",
	)

	assert.ErrorIs(t, err, category.ErrDoesNotExist)
	assert.False(t, repository.saveCalled, "provider should not be saved when category does not exist")
}

func TestRegisterProviderWithWrongCategoryID(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		2,
		"",
	)

	assert.ErrorIs(t, err, category.ErrDoesNotExist)
	assert.False(t, repository.saveCalled, "provider should not be saved when category id does not exist")
}

func TestFilterProvidersByCategoryID(t *testing.T) {
	providerCategory := existingCategory()
	providerToReturn, err := provider.NewProvider("auth0|ana", "ana@example.com", "Ana", "Perez", &providerCategory, "profile-photo-file-id")
	require.NoError(t, err)
	providerToReturn.ID = 1
	repository := &providerRepositoryMock{
		providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*providerToReturn},
		},
	}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{profilePhotoURLsByFile: map[string]string{"profile-photo-file-id": "https://cdn/profile-photo.jpg"}}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	providers, err := providerManager.FilterProvidersByCategoryID(context.Background(), 1)

	require.NoError(t, err)
	assert.Len(t, providers, 1)
	assert.Equal(t, 1, providers[0].ID)
	assert.Equal(t, "Ana", providers[0].Name)
	assert.Equal(t, "Perez", providers[0].Surname)
	assert.Equal(t, "Plomería", providers[0].CategoryName)
	assert.Equal(t, "https://cdn/profile-photo.jpg", providers[0].ProfilePhotoURL)
	assert.Equal(t, []string{"profile-photo-file-id"}, profilePhotoValidator.resolvedFileIDs)
	assert.True(t, repository.findByCategoryIDCalled, "providers should be searched by category id")
	assert.Equal(t, providerCategory.ID, repository.requestedCategoryID)
}

func TestFilterProvidersByCategoryIDFindsExistingCategory(t *testing.T) {
	repository := &providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{}}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	_, err := providerManager.FilterProvidersByCategoryID(context.Background(), 1)

	require.NoError(t, err)
	assert.True(t, repository.findByCategoryIDCalled, "providers should be searched when category exists")
	assert.Equal(t, 1, repository.requestedCategoryID)
}

func TestFilterProvidersByCategoryIDReturnsEmptyListWhenNoProvidersExist(t *testing.T) {
	repository := &providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{}}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	providers, err := providerManager.FilterProvidersByCategoryID(context.Background(), 1)

	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestFilterProvidersByCategoryIDReturnsRepositoryError(t *testing.T) {
	expectedErr := errors.New("find providers")
	repository := &providerRepositoryMock{findByCategoryIDErr: expectedErr}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	providers, err := providerManager.FilterProvidersByCategoryID(context.Background(), 1)

	assert.Nil(t, providers)
	assert.ErrorIs(t, err, expectedErr)
}

func TestFilterProvidersByCategoryIDWrapsProfilePhotoURLResolutionError(t *testing.T) {
	expectedErr := errors.New("resolve urls")
	providerCategory := existingCategory()
	providerToReturn, err := provider.NewProvider("auth0|ana", "ana@example.com", "Ana", "Perez", &providerCategory, "profile-photo-file-id")
	require.NoError(t, err)
	repository := &providerRepositoryMock{
		providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*providerToReturn},
		},
	}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{resolveErr: expectedErr})

	providers, err := providerManager.FilterProvidersByCategoryID(context.Background(), 1)

	assert.Nil(t, providers)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "resolving provider profile photo urls")
}

func TestFilterProvidersByCategoryIDRequiresCategoryID(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	providers, err := providerManager.FilterProvidersByCategoryID(context.Background(), 0)

	assert.ErrorIs(t, err, category.ErrIDRequired)
	assert.Nil(t, providers)
	assert.False(t, repository.findByCategoryIDCalled, "providers should not be searched when category id is missing")
}

func TestFilterProvidersByCategoryIDRequiresExistingCategory(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	providerManager := provider.NewService(repository, categoryFinder, &profilePhotoValidatorMock{})

	providers, err := providerManager.FilterProvidersByCategoryID(context.Background(), 999)

	assert.ErrorIs(t, err, category.ErrDoesNotExist)
	assert.Nil(t, providers)
	assert.False(t, repository.findByCategoryIDCalled, "providers should not be searched when category does not exist")
}

func TestRegisterProviderRequiresProfilePhoto(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{err: filedomain.ErrProfilePhotoRequired}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"",
	)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoRequired)
	assert.False(t, repository.saveCalled, "provider should not be saved without profile photo")
}

func TestRegisterProviderRejectsUnavailableProfilePhoto(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{err: filedomain.ErrProfilePhotoNotAvailable}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"file-id",
	)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
	assert.False(t, repository.saveCalled, "provider should not be saved with unavailable profile photo")
	assert.Equal(t, "auth0|ana", profilePhotoValidator.validatedAuthID)
	assert.Equal(t, "file-id", profilePhotoValidator.validatedFileID)
}

func TestRegisterProviderMapsUnexpectedProfilePhotoValidationError(t *testing.T) {
	repository := &providerRepositoryMock{}
	categoryFinder := categoryFinderWithExistingCategory()
	profilePhotoValidator := &profilePhotoValidatorMock{err: errors.New("storage unavailable")}
	providerManager := provider.NewService(repository, categoryFinder, profilePhotoValidator)

	_, err := providerManager.RegisterProvider(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		1,
		"file-id",
	)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
	assert.False(t, repository.saveCalled, "provider should not be saved with unavailable profile photo")
}
