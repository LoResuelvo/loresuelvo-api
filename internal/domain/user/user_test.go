package user_test

import (
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
)

func TestUserOwnsProfilePhotoImage(t *testing.T) {
	profilePhoto := &filedomain.Image{FileID: "profile-photo", OriginalName: "profile.jpg"}

	createdUser, err := user.New("auth0|123", "Andres", "Colina", "andres@example.com", "consumer", profilePhoto)

	assert.NoError(t, err)
	assert.Same(t, profilePhoto, createdUser.ProfilePhoto)
}

func TestUserReturnsErrorOnInvalidEmailFormat(t *testing.T) {
	user, err := user.New("auth0|123", "Andres", "Colina", "invalid-email", "provider", nil)
	assert.Error(t, err)
	assert.Nil(t, user)
}
