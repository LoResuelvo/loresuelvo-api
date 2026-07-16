package user_test

import (
	"context"
	"errors"
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userRepositoryMock struct {
	user               user.User
	findByAuthIDCalled bool
}

func (repository *userRepositoryMock) Save(_ context.Context, userToSave user.User) (user.User, error) {
	return userToSave, nil
}

func (repository *userRepositoryMock) FindByEmail(email string) bool {
	return false
}

func (repository *userRepositoryMock) FindByAuthID(id string) (user.User, error) {
	repository.findByAuthIDCalled = true
	if repository.user == nil || repository.user.AuthID() != id {
		return nil, user.ErrNotFound
	}
	return repository.user, nil
}

type profilePhotoURLResolverMock struct {
	resolvedFileID string
	url            string
	err            error
}

func (resolver *profilePhotoURLResolverMock) ResolvePublicURL(_ context.Context, fileID string) (string, error) {
	resolver.resolvedFileID = fileID
	return resolver.url, resolver.err
}

func TestGetExistingUser(t *testing.T) {
	repository := &userRepositoryMock{user: user.RehydrateBaseUser(0, "auth0|ana", "", "", "", "", nil)}
	userManager := user.NewService(repository, &profilePhotoURLResolverMock{})

	currentUser, err := userManager.GetCurrentUser(context.Background(), "auth0|ana")

	require.NoError(t, err)
	require.NotNil(t, currentUser)
	assert.Equal(t, "auth0|ana", currentUser.AuthID())
	assert.True(t, repository.findByAuthIDCalled, "user should be searched by auth ID")
}

func TestGetNonExistingUser(t *testing.T) {
	repository := &userRepositoryMock{}
	userManager := user.NewService(repository, &profilePhotoURLResolverMock{})

	currentUser, err := userManager.GetCurrentUser(context.Background(), "auth0|ana")

	require.Error(t, err)
	require.Nil(t, currentUser)
	assert.True(t, repository.findByAuthIDCalled, "user should be searched by auth ID")
}

func TestGetCurrentUserResolvesProfilePhotoURL(t *testing.T) {
	profilePhoto := &filedomain.Image{FileID: "photo-id", OriginalName: "profile.jpg"}
	repository := &userRepositoryMock{user: user.RehydrateBaseUser(0, "auth0|ana", "", "", "", "", profilePhoto)}
	resolver := &profilePhotoURLResolverMock{url: "https://files/profile.jpg"}
	userManager := user.NewService(repository, resolver)

	currentUser, err := userManager.GetCurrentUser(context.Background(), "auth0|ana")

	require.NoError(t, err)
	assert.Equal(t, "photo-id", resolver.resolvedFileID)
	assert.Equal(t, "https://files/profile.jpg", currentUser.ProfilePhoto().URL)
}

func TestGetCurrentUserReturnsProfilePhotoURLResolutionError(t *testing.T) {
	expectedErr := errors.New("storage unavailable")
	repository := &userRepositoryMock{user: user.RehydrateBaseUser(0, "auth0|ana", "", "", "", "", &filedomain.Image{FileID: "photo-id"})}
	userManager := user.NewService(repository, &profilePhotoURLResolverMock{err: expectedErr})

	currentUser, err := userManager.GetCurrentUser(context.Background(), "auth0|ana")

	require.ErrorIs(t, err, expectedErr)
	assert.Nil(t, currentUser)
}
