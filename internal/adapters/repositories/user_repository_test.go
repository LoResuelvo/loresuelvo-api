package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserRepositoryTest(t *testing.T) *repositories.UserRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		database.Close()
	})

	return repositories.NewUserRepository(database)
}

func validUser() *user.User {
	return user.New("auth0|josue", "Josue", "el pro", "josugod@gmail.com", "user")
}

func TestUserRepositoryCanSaveAUser(t *testing.T) {
	repo := newUserRepositoryTest(t)
	user := validUser()

	err := repo.Save(*user)

	assert.NoError(t, err)
	exists := repo.FindByEmail(user.Email)
	assert.True(t, exists, "User should be saved on database")
}

func TestUserRepositoryCanDeleteAllUsers(t *testing.T) {
	repo := newUserRepositoryTest(t)

	_ = repo.Save(*validUser())

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validUser().Email)
	assert.False(t, exists, "All users should be deleted from database")
}

func TestUserRepositoryCanFindByEmail(t *testing.T) {
	repo := newUserRepositoryTest(t)
	user := validUser()

	_ = repo.Save(*user)

	assert.True(t, repo.FindByEmail(user.Email), "User should be found by email")
}

func TestUserRepositoryFindByEmailReturnsFalseIfUserDoesNotExist(t *testing.T) {
	repo := newUserRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}
