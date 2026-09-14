package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepositorySavesAdmin(t *testing.T) {
	repository, _ := newUserRepositoryTest(t)
	expected := newAdmin(t, "auth0|admin", "admin@example.com")

	saved, err := repository.Save(context.Background(), expected)

	require.NoError(t, err)
	assert.Positive(t, saved.ID())
	assert.True(t, repository.FindByEmail(expected.Email()))
}

func TestUserRepositoryHydratesAdminByAuthID(t *testing.T) {
	repository, _ := newUserRepositoryTest(t)
	expected := newAdmin(t, "auth0|admin", "admin@example.com")
	saved, err := repository.Save(context.Background(), expected)
	require.NoError(t, err)

	found, err := repository.FindByAuthID(expected.AuthID())
	require.NoError(t, err)
	foundAdmin, ok := found.(*admin.Admin)
	require.True(t, ok, "expected admin, got %T", found)
	assert.Equal(t, saved.ID(), foundAdmin.ID())
	assert.Equal(t, expected.AuthID(), foundAdmin.AuthID())
	assert.Equal(t, expected.Email(), foundAdmin.Email())
	assert.Equal(t, expected.Name(), foundAdmin.Name())
	assert.Equal(t, expected.Surname(), foundAdmin.Surname())
	assert.Equal(t, admin.Role, foundAdmin.Role())
}

func TestUserRepositoryDoesNotFindAdminByDifferentAuthID(t *testing.T) {
	repository, _ := newUserRepositoryTest(t)
	expected := newAdmin(t, "auth0|admin", "admin@example.com")
	_, err := repository.Save(context.Background(), expected)
	require.NoError(t, err)

	found, err := repository.FindByAuthID("auth0|another-identity")

	assert.Nil(t, found)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUserRepositoryRejectsAdminWithMismatchedRole(t *testing.T) {
	repository, _ := newUserRepositoryTest(t)
	invalidAdmin := admin.RehydrateAdmin(user.RehydrateBaseUser(
		0,
		"auth0|admin",
		"admin@example.com",
		"Ana",
		"Perez",
		"consumer",
		nil,
	))

	saved, err := repository.Save(context.Background(), invalidAdmin)

	assert.Nil(t, saved)
	assert.ErrorContains(t, err, "admin has role")
	assert.False(t, repository.FindByEmail(invalidAdmin.Email()))
}

func newAdmin(t *testing.T, authID, email string) *admin.Admin {
	t.Helper()

	createdAdmin, err := admin.NewAdmin(authID, email, "Ana", "Perez", nil)
	require.NoError(t, err)
	return createdAdmin
}

var _ user.User = (*admin.Admin)(nil)
