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

func newConsumerRepositoryTest(t *testing.T) *repositories.ConsumerRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		database.Close()
	})
	userRepository := repositories.NewUserRepository(database)
	return repositories.NewConsumerRepository(database, userRepository)
}

func validConsumer() consumer.Consumer {
	return consumer.NewConsumer("auth0|josue", "josugod@gmail.com", "Josue", "el pro")
}

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	err := repo.Save(consumer)

	assert.NoError(t, err)
	exists := repo.FindByEmail(consumer.User.Email)
	assert.True(t, exists, "Consumer should be saved on database")
}

func TestConsumerRepositoryCanDeleteAllConsumers(t *testing.T) {
	repo := newConsumerRepositoryTest(t)

	_ = repo.Save(validConsumer())

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validConsumer().User.Email)
	assert.False(t, exists, "All consumers should be deleted from database")
}

func TestConsumerRepositoryCanFindByEmail(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	err := repo.Save(consumer)

	assert.NoError(t, err, "saving consumer should not return an error")
	assert.True(t, repo.FindByEmail(consumer.User.Email), "Consumer should be found by email")
}

func TestConsumerRepositoryFindByEmailReturnsFalseIfConsumerDoesNotExist(t *testing.T) {
	repo := newConsumerRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}
