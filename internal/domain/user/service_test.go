package user_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userRepositoryMock struct {
	user               *user.User
	findByAuthIDCalled bool
}

func (repository *userRepositoryMock) Save(user user.User) error {
	return nil
}

func (repository *userRepositoryMock) FindByEmail(email string) bool {
	return false
}

func (repository *userRepositoryMock) FindByAuthID(id string) (*user.User, error) {
	repository.findByAuthIDCalled = true
	if repository.user == nil || repository.user.AuthID != id {
		return nil, user.ErrNotFound
	}
	return repository.user, nil
}

func TestGetExistingUser(t *testing.T) {
	repository := &userRepositoryMock{user: &user.User{AuthID: "auth0|ana"}}
	userManager := user.NewService(repository)

	currentUser, err := userManager.GetCurrentUser("auth0|ana")

	require.NoError(t, err)
	require.NotNil(t, currentUser)
	assert.Equal(t, "auth0|ana", currentUser.AuthID)
	assert.True(t, repository.findByAuthIDCalled, "user should be searched by auth ID")
}

func TestGetNonExistingUser(t *testing.T) {
	repository := &userRepositoryMock{}
	userManager := user.NewService(repository)

	currentUser, err := userManager.GetCurrentUser("auth0|ana")

	require.Error(t, err)
	require.Nil(t, currentUser)
	assert.True(t, repository.findByAuthIDCalled, "user should be searched by auth ID")
}
