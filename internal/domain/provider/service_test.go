package provider_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
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

	if err != nil {
		t.Fatalf("se esperaba registrar el consumidor sin error, pero se obtuvo: %v", err)
	}

	if repository.savedProvider.Auth0ID != "auth0|ana" {
		t.Fatalf("se esperaba guardar el consumidor con auth0_id auth0|ana, pero se obtuvo %s", repository.savedProvider.Auth0ID)
	}

	if repository.savedProvider.Email != "ana@example.com" {
		t.Fatalf("se esperaba guardar el consumidor con email ana@example.com, pero se obtuvo %s", repository.savedProvider.Email)
	}
}

func TestRegisterProviderWithEmailWithoutArroba(t *testing.T) {
	repository := &providerRepositoryMock{}
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"anaexample.com",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"})

	if err != validator.ErrInvalidEmailFormat {
		t.Fatalf("se esperaba un error de formato de correo inválido, pero se obtuvo: %v", err)
	}

	if repository.saveCalled {
		t.Fatal("no se esperaba guardar el consumidor cuando el correo es inválido")
	}
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

	if err != validator.ErrInvalidEmailFormat {
		t.Fatalf("se esperaba un error de formato de correo inválido, pero se obtuvo: %v", err)
	}

	if repository.saveCalled {
		t.Fatal("no se esperaba guardar el consumidor cuando el correo es inválido")
	}
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

	if err != validator.ErrInvalidEmailFormat {
		t.Fatalf("se esperaba un error de formato de correo inválido, pero se obtuvo: %v", err)
	}

	if repository.saveCalled {
		t.Fatal("no se esperaba guardar el consumidor cuando el correo es inválido")
	}
}

func TestRegisterProviderWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &providerRepositoryMock{existsByEmailValue: true}
	providerManager := provider.NewService(repository)

	err := providerManager.RegisterProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		[]string{"Palermo", "Belgrano"})

	if err != validator.ErrEmailAlreadyRegistered {
		t.Fatalf("Expected %v, but got %v", validator.ErrEmailAlreadyRegistered, err)
	}

	if repository.saveCalled {
		t.Fatal("was not expected to save the provider when the email is already registered")
	}

	if !repository.findByEmailCalled {
		t.Fatal("was expected to check if the email is already registered")
	}
}
