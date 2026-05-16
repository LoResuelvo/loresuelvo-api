package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
)

func newConsumerRepositoryTest(t *testing.T) *repositories.ConsumerRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	if err != nil {
		t.Fatalf("error al conectar a la DB de pruebas: %v", err)
	}

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM consumers")
		database.Close()
	})

	return repositories.NewConsumerRepository(database)
}

func validConsumer() consumer.Consumer {
	return consumer.NewConsumer("auth0|josue", "josugod@gmail.com", "Josue", "el pro")
}

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	err := repo.Save(consumer)

	assert.NoError(t, err)
	exists := repo.FindByEmail(consumer.Email)
	assert.True(t, exists, "Consumer should be saved on database")
}

func TestConsumerRepositoryCanDeleteAllConsumers(t *testing.T) {
	repo := newConsumerRepositoryTest(t)

	_ = repo.Save(validConsumer())

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validConsumer().Email)
	assert.False(t, exists, "All consumers should be deleted from database")
}

func TestConsumerRepositoryCanFindByEmail(t *testing.T) {
	repo := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	_ = repo.Save(consumer)

	assert.True(t, repo.FindByEmail(consumer.Email), "Consumer should be found by email")
}

func TestConsumerRepositoryFindByEmailReturnsFalseIfConsumerDoesNotExist(t *testing.T) {
	repo := newConsumerRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}
