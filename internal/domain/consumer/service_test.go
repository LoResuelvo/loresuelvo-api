package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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
	repository.savedConsumer.ID = 10
	userToSave.Base().ID = 10
	repository.saveCalled = true
	return userToSave, nil
}

func (repository *consumerRepositoryMock) FindByEmail(string) bool {
	repository.findByEmailCalled = true
	return repository.existsByEmailValue
}

type profilePhotoServiceMock struct {
	validateErr     error
	resolveErr      error
	resolvedURL     string
	validatedAuthID string
	validatedFileID string
	resolvedFileID  string
}

func (service *profilePhotoServiceMock) ValidateProfilePhoto(_ context.Context, authID, fileID string) error {
	service.validatedAuthID = authID
	service.validatedFileID = fileID
	return service.validateErr
}

func (service *profilePhotoServiceMock) ResolvePublicURL(_ context.Context, fileID string) (string, error) {
	service.resolvedFileID = fileID
	return service.resolvedURL, service.resolveErr
}

func TestRegisterConsumerWithProfilePhoto(t *testing.T) {
	repository := &consumerRepositoryMock{}
	files := &profilePhotoServiceMock{resolvedURL: "https://cdn/profile.jpg"}
	consumerManager := consumer.NewService(repository, files)

	created, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "profile-file-id",
	)

	assert.NoError(t, err)
	assert.Equal(t, 10, created.ID)
	assert.Equal(t, "https://cdn/profile.jpg", created.ProfilePhoto.URL)
	assert.Equal(t, "profile-file-id", repository.savedConsumer.ProfilePhoto.FileID)
	assert.Equal(t, "auth0|ana", files.validatedAuthID)
	assert.Equal(t, "profile-file-id", files.validatedFileID)
	assert.Equal(t, "profile-file-id", files.resolvedFileID)
}

func TestRegisterConsumerWithoutProfilePhoto(t *testing.T) {
	repository := &consumerRepositoryMock{}
	files := &profilePhotoServiceMock{}
	consumerManager := consumer.NewService(repository, files)

	created, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
	)

	assert.NoError(t, err)
	assert.Nil(t, created.ProfilePhoto)
	assert.Nil(t, repository.savedConsumer.ProfilePhoto)
	assert.Empty(t, files.validatedFileID)
	assert.Empty(t, files.resolvedFileID)
}

func TestRegisterConsumerRejectsUnavailableProfilePhoto(t *testing.T) {
	repository := &consumerRepositoryMock{}
	files := &profilePhotoServiceMock{validateErr: filedomain.ErrProfilePhotoNotAvailable}
	consumerManager := consumer.NewService(repository, files)

	created, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "unavailable",
	)

	assert.Nil(t, created)
	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
	assert.False(t, repository.saveCalled)
}

func TestRegisterConsumerMapsUnexpectedProfilePhotoValidationError(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := consumer.NewService(repository, &profilePhotoServiceMock{validateErr: errors.New("storage unavailable")})

	_, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "profile-file-id",
	)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
	assert.False(t, repository.saveCalled)
}

func TestRegisterConsumerWithInvalidEmail(t *testing.T) {
	for _, invalidEmail := range []string{"anaexample.com", "ana@", "@example.com"} {
		t.Run(invalidEmail, func(t *testing.T) {
			repository := &consumerRepositoryMock{}
			consumerManager := consumer.NewService(repository, &profilePhotoServiceMock{})

			_, err := consumerManager.RegisterConsumer(
				context.Background(), "auth0|ana", invalidEmail, "Ana", "Perez", "",
			)

			assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
			assert.False(t, repository.saveCalled)
		})
	}
}

func TestRegisterConsumerWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &consumerRepositoryMock{existsByEmailValue: true}
	consumerManager := consumer.NewService(repository, &profilePhotoServiceMock{})

	_, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
	)

	assert.ErrorIs(t, err, validator.ErrEmailAlreadyRegistered)
	assert.False(t, repository.saveCalled)
	assert.True(t, repository.findByEmailCalled)
}
