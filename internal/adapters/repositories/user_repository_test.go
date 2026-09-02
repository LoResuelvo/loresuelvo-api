package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserRepositoryTest(t *testing.T) (*repositories.UserRepository, *sql.DB) {
	t.Helper()

	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		database.Close()
	})

	return repositories.NewUserRepository(database), database
}

func validUser(t *testing.T, database *sql.DB) *consumer.Consumer {
	return consumerWithAddress(t, database, "auth0|josue", "josugod@gmail.com", "Josue", "el pro")
}

func TestUserRepositoryCanSaveAUser(t *testing.T) {
	repo, database := newUserRepositoryTest(t)
	user := validUser(t, database)

	_, err := repo.Save(context.Background(), user)

	assert.NoError(t, err)
	exists := repo.FindByEmail(user.Email())
	assert.True(t, exists, "User should be saved on database")
}

func TestUserRepositoryCanDeleteAllUsers(t *testing.T) {
	repo, database := newUserRepositoryTest(t)

	_, _ = repo.Save(context.Background(), validUser(t, database))

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validUser(t, database).Email())
	assert.False(t, exists, "All users should be deleted from database")
}

func TestUserRepositoryCanFindByEmail(t *testing.T) {
	repo, database := newUserRepositoryTest(t)
	user := validUser(t, database)

	_, _ = repo.Save(context.Background(), user)

	assert.True(t, repo.FindByEmail(user.Email()), "User should be found by email")
}

func TestUserRepositoryCanFindPolymorphicUserByID(t *testing.T) {
	repo, database := newUserRepositoryTest(t)
	expected := validUser(t, database)
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
	repo, _ := newUserRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}
