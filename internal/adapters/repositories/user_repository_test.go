package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
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

func validUser() *consumer.Consumer {
	user, _ := consumer.NewConsumer("auth0|josue", "josugod@gmail.com", "Josue", "el pro", nil)
	return user
}

func TestUserRepositoryCanSaveAUser(t *testing.T) {
	repo := newUserRepositoryTest(t)
	user := validUser()

	_, err := repo.Save(context.Background(), user)

	assert.NoError(t, err)
	exists := repo.FindByEmail(user.Email())
	assert.True(t, exists, "User should be saved on database")
}

func TestUserRepositoryCanDeleteAllUsers(t *testing.T) {
	repo := newUserRepositoryTest(t)

	_, _ = repo.Save(context.Background(), validUser())

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validUser().Email())
	assert.False(t, exists, "All users should be deleted from database")
}

func TestUserRepositoryCanFindByEmail(t *testing.T) {
	repo := newUserRepositoryTest(t)
	user := validUser()

	_, _ = repo.Save(context.Background(), user)

	assert.True(t, repo.FindByEmail(user.Email()), "User should be found by email")
}

func TestUserRepositoryCanFindPolymorphicUserByID(t *testing.T) {
	repo := newUserRepositoryTest(t)
	expected := validUser()
	saved, err := repo.Save(context.Background(), expected)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), saved.ID())

	require.NoError(t, err)
	foundConsumer, ok := found.(*consumer.Consumer)
	require.True(t, ok, "expected consumer, got %T", found)
	assert.Equal(t, saved.ID(), foundConsumer.ID())
	assert.Equal(t, saved.AuthID(), foundConsumer.AuthID())
	assert.Equal(t, consumer.Role, foundConsumer.Role())
}

func TestUserRepositoryFindByEmailReturnsFalseIfUserDoesNotExist(t *testing.T) {
	repo := newUserRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}
