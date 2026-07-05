package consumer_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/stretchr/testify/assert"
)

type consumerRepositoryMock struct {
	savedConsumer      consumer.Consumer
	saveCalled         bool
	existsByEmailValue bool
	findByEmailCalled  bool
}

func (repository *consumerRepositoryMock) Save(_ context.Context, userToSave user.User) (user.User, error) {
	repository.savedConsumer = *userToSave.(*consumer.Consumer)
	repository.saveCalled = true
	return userToSave, nil
}

func (repository *consumerRepositoryMock) FindByEmail(email string) bool {
	repository.findByEmailCalled = true
	return repository.existsByEmailValue
}

func TestRegisterConsumerWithValidData(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := consumer.NewService(repository)

	err := consumerManager.RegisterConsumer(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
	)

	assert.NoError(t, err)
	assert.Equal(t, "auth0|ana", repository.savedConsumer.BaseUser.AuthID)
	assert.Equal(t, "ana@example.com", repository.savedConsumer.BaseUser.Email)
}

func TestRegisterConsumerWithEmailWithoutArroba(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := consumer.NewService(repository)

	err := consumerManager.RegisterConsumer(
		context.Background(),
		"auth0|ana",
		"anaexample.com",
		"Ana",
		"Perez",
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "consumer should not be saved when email is invalid")
}

func TestRegisterConsumerWithEmailWithoutDomain(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := consumer.NewService(repository)

	err := consumerManager.RegisterConsumer(
		context.Background(),
		"auth0|ana",
		"ana@",
		"Ana",
		"Perez",
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "consumer should not be saved when email is invalid")
}

func TestRegisterConsumerWithEmailWithoutName(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := consumer.NewService(repository)

	err := consumerManager.RegisterConsumer(
		context.Background(),
		"auth0|ana",
		"@example.com",
		"Ana",
		"Perez",
	)

	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
	assert.False(t, repository.saveCalled, "consumer should not be saved when email is invalid")
}

func TestRegisterConsumerWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &consumerRepositoryMock{existsByEmailValue: true}
	consumerManager := consumer.NewService(repository)

	err := consumerManager.RegisterConsumer(
		context.Background(),
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
	)

	assert.ErrorIs(t, err, validator.ErrEmailAlreadyRegistered)
	assert.False(t, repository.saveCalled, "consumer should not be saved when email is already registered")
	assert.True(t, repository.findByEmailCalled, "email registration should be checked")
}
