package user_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
)

func TestUserReturnsErrorOnInvalidEmailFormat(t *testing.T) {
	user, err := user.New("auth0|123", "Andres", "Colina", "invalid-email", "provider", "")
	assert.Error(t, err)
	assert.Nil(t, user)
}
