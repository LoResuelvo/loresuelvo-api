package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
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
	repository.savedConsumer.SetPersistenceID(10)
	userToSave.SetPersistenceID(10)
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

type addressResolverMock struct {
	location consumer.GeoPoint
	err      error
}

func (resolver *addressResolverMock) Resolve(_ context.Context, _ consumer.Address) (consumer.GeoPoint, error) {
	return resolver.location, resolver.err
}

type coverageZoneResolverMock struct {
	zone *coveragezone.CoverageZone
	err  error
}

func (resolver *coverageZoneResolverMock) Resolve(_ context.Context, _ consumer.GeoPoint) (*coveragezone.CoverageZone, error) {
	return resolver.zone, resolver.err
}

func validRegistrationAddress() consumer.Address {
	return consumer.Address{Street: "Av. Rivadavia", StreetNumber: "5100"}
}

func newConsumerService(repository *consumerRepositoryMock, files *profilePhotoServiceMock) *consumer.Service {
	return consumer.NewService(
		repository,
		files,
		&addressResolverMock{location: consumer.GeoPoint{Latitude: -34.62, Longitude: -58.43}},
		&coverageZoneResolverMock{zone: &coveragezone.CoverageZone{ID: 6, Enabled: true}},
	)
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
	consumerManager := newConsumerService(repository, files)

	created, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "profile-file-id",
		validRegistrationAddress(),
	)

	assert.NoError(t, err)
	assert.Equal(t, 10, created.ID())
	assert.Equal(t, "https://cdn/profile.jpg", created.ProfilePhoto().URL)
	assert.Equal(t, "profile-file-id", repository.savedConsumer.ProfilePhoto().FileID)
	assert.Equal(t, "auth0|ana", files.validatedAuthID)
	assert.Equal(t, "profile-file-id", files.validatedFileID)
	assert.Equal(t, "profile-file-id", files.resolvedFileID)
}

func TestRegisterConsumerWithoutProfilePhoto(t *testing.T) {
	repository := &consumerRepositoryMock{}
	files := &profilePhotoServiceMock{}
	consumerManager := newConsumerService(repository, files)

	created, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
		validRegistrationAddress(),
	)

	assert.NoError(t, err)
	assert.Nil(t, created.ProfilePhoto())
	assert.Nil(t, repository.savedConsumer.ProfilePhoto())
	assert.Empty(t, files.validatedFileID)
	assert.Empty(t, files.resolvedFileID)
}

func TestRegisterConsumerRejectsUnavailableProfilePhoto(t *testing.T) {
	repository := &consumerRepositoryMock{}
	files := &profilePhotoServiceMock{validateErr: filedomain.ErrProfilePhotoNotAvailable}
	consumerManager := newConsumerService(repository, files)

	created, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "unavailable",
		validRegistrationAddress(),
	)

	assert.Nil(t, created)
	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
	assert.False(t, repository.saveCalled)
}

func TestRegisterConsumerMapsUnexpectedProfilePhotoValidationError(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := newConsumerService(repository, &profilePhotoServiceMock{validateErr: errors.New("storage unavailable")})

	_, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "profile-file-id",
		validRegistrationAddress(),
	)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
	assert.False(t, repository.saveCalled)
}

func TestRegisterConsumerWithInvalidEmail(t *testing.T) {
	for _, invalidEmail := range []string{"anaexample.com", "ana@", "@example.com"} {
		t.Run(invalidEmail, func(t *testing.T) {
			repository := &consumerRepositoryMock{}
			consumerManager := newConsumerService(repository, &profilePhotoServiceMock{})

			_, err := consumerManager.RegisterConsumer(
				context.Background(), "auth0|ana", invalidEmail, "Ana", "Perez", "",
				validRegistrationAddress(),
			)

			assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
			assert.False(t, repository.saveCalled)
		})
	}
}

func TestRegisterConsumerWithAlreadyRegisteredEmail(t *testing.T) {
	repository := &consumerRepositoryMock{existsByEmailValue: true}
	consumerManager := newConsumerService(repository, &profilePhotoServiceMock{})

	_, err := consumerManager.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
		validRegistrationAddress(),
	)

	assert.ErrorIs(t, err, validator.ErrEmailAlreadyRegistered)
	assert.False(t, repository.saveCalled)
	assert.True(t, repository.findByEmailCalled)
}

func TestRegisterConsumerRejectsMissingAddressBeforeResolvingLocation(t *testing.T) {
	repository := &consumerRepositoryMock{}
	addressResolver := &addressResolverMock{}
	service := consumer.NewService(
		repository,
		&profilePhotoServiceMock{},
		addressResolver,
		&coverageZoneResolverMock{zone: &coveragezone.CoverageZone{ID: 6, Enabled: true}},
	)

	created, err := service.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
		consumer.Address{},
	)

	assert.Nil(t, created)
	assert.ErrorIs(t, err, consumer.ErrAddressRequired)
	assert.False(t, repository.saveCalled)
	assert.Empty(t, addressResolver.location)
}

func TestRegisterConsumerDoesNotPersistWhenAddressResolutionFails(t *testing.T) {
	repository := &consumerRepositoryMock{}
	service := consumer.NewService(
		repository,
		&profilePhotoServiceMock{},
		&addressResolverMock{err: consumer.ErrAddressNotValidated},
		&coverageZoneResolverMock{zone: &coveragezone.CoverageZone{ID: 6, Enabled: true}},
	)

	created, err := service.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
		validRegistrationAddress(),
	)

	assert.Nil(t, created)
	assert.ErrorIs(t, err, consumer.ErrAddressNotValidated)
	assert.False(t, repository.saveCalled)
}

func TestRegisterConsumerRejectsUnavailableCoverageZoneWithoutPersisting(t *testing.T) {
	repository := &consumerRepositoryMock{}
	service := consumer.NewService(
		repository,
		&profilePhotoServiceMock{},
		&addressResolverMock{location: consumer.GeoPoint{Latitude: -34.62, Longitude: -58.43}},
		&coverageZoneResolverMock{err: coveragezone.ErrDoesNotExist},
	)

	created, err := service.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "",
		validRegistrationAddress(),
	)

	assert.Nil(t, created)
	assert.ErrorIs(t, err, consumer.ErrCoverageZoneNotAvailable)
	assert.False(t, repository.saveCalled)
}

func TestRegisterConsumerPersistsResolvedAddressData(t *testing.T) {
	repository := &consumerRepositoryMock{}
	point := consumer.GeoPoint{Latitude: -34.62, Longitude: -58.43}
	service := consumer.NewService(
		repository,
		&profilePhotoServiceMock{},
		&addressResolverMock{location: point},
		&coverageZoneResolverMock{zone: &coveragezone.CoverageZone{ID: 6, Enabled: true}},
	)
	address := consumer.Address{Street: " Av. Rivadavia ", StreetNumber: " 5100 ", Floor: " 4 ", Unit: " B "}
	normalizedAddress := consumer.Address{Street: "Av. Rivadavia", StreetNumber: "5100", Floor: "4", Unit: "B"}

	created, err := service.RegisterConsumer(
		context.Background(), "auth0|ana", "ana@example.com", "Ana", "Perez", "", address,
	)

	assert.NoError(t, err)
	assert.Equal(t, normalizedAddress, repository.savedConsumer.Address())
	assert.Equal(t, point, repository.savedConsumer.Location())
	assert.Equal(t, 6, repository.savedConsumer.CoverageZoneID())
	assert.Equal(t, normalizedAddress, created.Address())
}
