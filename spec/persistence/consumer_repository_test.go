package persistence_test

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/postgres"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/assert"
)

func newConsumerRepositoryTest(t *testing.T) (*postgres.ConsumerRepository, sqlmock.Sqlmock) {
	t.Helper()

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("no se pudo crear sqlmock: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return postgres.NewConsumerRepository(database), mock
}

func validConsumer() consumer.Consumer {
	return consumer.NewConsumer("josugod@gmail.com", "Josue", "el pro", "SoyUnCrack123")
}

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo, mock := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO consumers (email, name, surname, password, created_on, updated_on)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`)).
		WithArgs(consumer.Email, consumer.Name, consumer.Surname, consumer.Password).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Save(consumer)

	assert.Nil(t, err)
	assert.Nil(t, mock.ExpectationsWereMet())
}

func TestConsumerRepositoryCanDeleteAllConsumers(t *testing.T) {
	repo, mock := newConsumerRepositoryTest(t)

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM consumers`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.DeleteAll()

	assert.Nil(t, err)
	assert.Nil(t, mock.ExpectationsWereMet())
}
