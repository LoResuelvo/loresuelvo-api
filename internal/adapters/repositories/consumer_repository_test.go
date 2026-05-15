package repositories_test

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/assert"
)

func newConsumerRepositoryTest(t *testing.T) (*repositories.ConsumerRepository, sqlmock.Sqlmock) {
	t.Helper()

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("no se pudo crear sqlmock: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return repositories.NewConsumerRepository(database), mock
}

func validConsumer() consumer.Consumer {
	return consumer.NewConsumer("auth0|josue", "josugod@gmail.com", "Josue", "el pro")
}

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo, mock := newConsumerRepositoryTest(t)
	consumer := validConsumer()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO consumers (auth0_id, email, name, surname, created_on, updated_on)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`)).
		WithArgs(consumer.Auth0ID, consumer.Email, consumer.Name, consumer.Surname).
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
