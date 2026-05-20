package provider_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/stretchr/testify/assert"
)

type providerRepositoryMock struct {
	savedProvider      provider.Provider
	saveCalled         bool
	existsByEmailValue bool
	findByEmailCalled  bool
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
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"},
	)

	assert.NoError(t, err)
	assert.Equal(t, "auth0|ana", repository.savedProvider.User.AuthID)
	assert.Equal(t, "ana@example.com", repository.savedProvider.User.Email)
}

func TestRegisterProviderWithEmailWithoutArroba(t *testing.T) {
	repository := &providerRepositoryMock{}
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"anaexample.com",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"},
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithEmailWithoutDomain(t *testing.T) {
	repository := &providerRepositoryMock{}
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"},
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithEmailWithoutName(t *testing.T) {
	repository := &providerRepositoryMock{}
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"@example.com",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"},
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is invalid")
}

func TestRegisterProviderWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &providerRepositoryMock{existsByEmailValue: true}
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"},
	)

	assert.ErrorIs(t, err, validator.ErrEmailAlreadyRegistered)
	assert.False(t, repository.saveCalled, "provider should not be saved when email is already registered")
	assert.True(t, repository.findByEmailCalled, "email registration should be checked")
}
