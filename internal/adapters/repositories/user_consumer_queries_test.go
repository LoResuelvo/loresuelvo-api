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

func newConsumerRepositoryTest(t *testing.T) *repositories.UserRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		database.Close()
	})

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	userRepository := repositories.NewUserRepository(database)
	return userRepository
}

func validConsumer() consumer.Consumer {
	consumer, _ := consumer.NewConsumer("auth0|josue", "josugod@gmail.com", "Josue", "el pro", nil)
	return *consumer
}

func consumerUser(value consumer.Consumer) *consumer.Consumer {
	return &value
}

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	_, err := repo.Save(context.Background(), consumerUser(consumer))

	assert.NoError(t, err)
	exists := repo.FindByEmail(consumer.Email())
	assert.True(t, exists, "Consumer should be saved on database")
}

func TestConsumerRepositoryCanDeleteAllConsumers(t *testing.T) {
	repo := newConsumerRepositoryTest(t)

	_, _ = repo.Save(context.Background(), consumerUser(validConsumer()))

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validConsumer().Email())
	assert.False(t, exists, "All consumers should be deleted from database")
}

func TestConsumerRepositoryCanFindByEmail(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	_, err := repo.Save(context.Background(), consumerUser(consumer))

	assert.NoError(t, err, "saving consumer should not return an error")
	assert.True(t, repo.FindByEmail(consumer.Email()), "Consumer should be found by email")
}

func TestConsumerRepositoryFindByEmailReturnsFalseIfConsumerDoesNotExist(t *testing.T) {
	repo := newConsumerRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}

func TestConsumerRepositoryCanFindIDByEmail(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	_, err := repo.Save(context.Background(), consumerUser(consumer))
	require.NoError(t, err, "saving consumer should not return an error")

	consumerID, err := repo.FindIDByEmail(consumer.Email())

	require.NoError(t, err)
	assert.NotZero(t, consumerID)
}

func TestConsumerRepositoryCanFindIDByAuthID(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	_, err := repo.Save(context.Background(), consumerUser(consumer))
	require.NoError(t, err, "saving consumer should not return an error")

	consumerID, err := repo.FindIDByAuthID(consumer.AuthID())

	require.NoError(t, err)
	assert.NotZero(t, consumerID)
}

func TestConsumerRepositoryCanFindByID(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumerToSave := validConsumer()

	_, err := repo.Save(context.Background(), consumerUser(consumerToSave))
	require.NoError(t, err, "saving consumer should not return an error")
	consumerID, err := repo.FindIDByEmail(consumerToSave.Email())
	require.NoError(t, err)

	foundConsumer, err := repo.FindConsumerByID(consumerID)

	require.NoError(t, err)
	assert.Equal(t, consumerID, foundConsumer.ID())
	require.NotNil(t, foundConsumer.BaseUser)
	assert.Equal(t, consumerToSave.AuthID(), foundConsumer.AuthID())
	assert.Equal(t, consumerToSave.Email(), foundConsumer.Email())
	assert.Equal(t, consumerToSave.Name(), foundConsumer.Name())
	assert.Equal(t, consumerToSave.Surname(), foundConsumer.Surname())
	assert.Equal(t, consumerToSave.Role(), foundConsumer.Role())
}
