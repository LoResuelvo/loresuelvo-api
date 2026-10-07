package installation

import (
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServiceDerivesRegistrationUserFromAuthenticatedActor(t *testing.T) {
	r := validRegistration()
	repository := new(repositoryMock)
	users := new(userFinderMock)
	users.On("FindByAuthID", "current-actor").Return(user.RehydrateBaseUser(10, "current-actor", "ana@example.com", "Ana", "Gomez", "consumer", nil), nil).Once()
	repository.On("FindByID", mock.Anything, r.ID).Return(nil, ErrNotFound).Once()
	repository.On("Save", mock.Anything, mock.MatchedBy(func(i *Installation) bool {
		return i.UserID == 10 && i.BindingID == r.BindingID && len(i.SecretHash) == 32
	})).Return(nil).Once()
	found, created, err := NewService(repository, users).Register(t.Context(), "current-actor", r)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, 10, found.UserID)
	users.AssertExpectations(t)
	repository.AssertExpectations(t)
}
func TestServicePropagatesConcurrentRegistrationConflict(t *testing.T) {
	r := validRegistration()
	repository := new(repositoryMock)
	users := new(userFinderMock)
	users.On("FindByAuthID", "actor").Return(user.RehydrateBaseUser(10, "actor", "ana@example.com", "Ana", "Gomez", "consumer", nil), nil).Once()
	repository.On("FindByID", mock.Anything, r.ID).Return(nil, ErrNotFound).Once()
	repository.On("Save", mock.Anything, mock.Anything).Return(ErrConflict).Once()
	_, _, err := NewService(repository, users).Register(t.Context(), "actor", r)
	require.ErrorIs(t, err, ErrConflict)
	repository.AssertExpectations(t)
}
func TestServiceRejectsUnknownAuthenticatedActor(t *testing.T) {
	users := new(userFinderMock)
	repository := new(repositoryMock)
	users.On("FindByAuthID", "unknown").Return(nil, errors.New("not registered")).Once()
	_, _, err := NewService(repository, users).Register(t.Context(), "unknown", validRegistration())
	require.ErrorIs(t, err, ErrForbidden)
	repository.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}
func TestServiceUnregistersAbsentInstallationIdempotently(t *testing.T) {
	r := validRegistration()
	repository := new(repositoryMock)
	users := new(userFinderMock)
	users.On("FindByAuthID", "actor").Return(user.RehydrateBaseUser(10, "actor", "ana@example.com", "Ana", "Gomez", "consumer", nil), nil).Once()
	repository.On("FindByID", mock.Anything, r.ID).Return(nil, ErrNotFound).Once()
	require.NoError(t, NewService(repository, users).Unregister(t.Context(), "actor", r.ID, r.Secret, r.BindingID))
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestServiceRejectsRegistrationForIncompatibleActorRole(t *testing.T) {
	r := validRegistration()
	repository := new(repositoryMock)
	users := new(userFinderMock)
	users.On("FindByAuthID", "provider-actor").Return(user.RehydrateBaseUser(20, "provider-actor", "juan@example.com", "Juan", "Perez", "provider", nil), nil).Once()
	repository.On("FindByID", mock.Anything, r.ID).Return(nil, ErrNotFound).Once()
	_, _, err := NewService(repository, users).Register(t.Context(), "provider-actor", r)
	require.ErrorIs(t, err, ErrForbidden)
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}
func TestServiceDoesNotAllowAnotherOwnerToUnregisterInstallation(t *testing.T) {
	r := validRegistration()
	stored, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	repository := new(repositoryMock)
	users := new(userFinderMock)
	users.On("FindByAuthID", "another-actor").Return(user.RehydrateBaseUser(20, "another-actor", "carla@example.com", "Carla", "Lopez", "consumer", nil), nil).Once()
	repository.On("FindByID", mock.Anything, r.ID).Return(stored, nil).Once()
	err = NewService(repository, users).Unregister(t.Context(), "another-actor", r.ID, r.Secret, r.BindingID)
	require.ErrorIs(t, err, ErrForbidden)
	require.True(t, stored.Enabled)
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}
