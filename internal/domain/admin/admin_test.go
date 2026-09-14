package admin_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdminCreatesAdminUser(t *testing.T) {
	createdAdmin, err := admin.NewAdmin(
		"auth0|admin",
		"admin@example.com",
		"Ana",
		"Perez",
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, "auth0|admin", createdAdmin.AuthID())
	assert.Equal(t, "admin@example.com", createdAdmin.Email())
	assert.Equal(t, "Ana", createdAdmin.Name())
	assert.Equal(t, "Perez", createdAdmin.Surname())
	assert.Equal(t, admin.Role, createdAdmin.Role())
}

func TestNewAdminRejectsInvalidEmail(t *testing.T) {
	createdAdmin, err := admin.NewAdmin(
		"auth0|admin",
		"invalid-email",
		"Ana",
		"Perez",
		nil,
	)

	assert.Nil(t, createdAdmin)
	assert.ErrorIs(t, err, validator.ErrInvalidEmailFormat)
}
