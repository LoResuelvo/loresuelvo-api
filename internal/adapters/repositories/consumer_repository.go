package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/assert"
)

type ConsumerRepository struct {
	db *sql.DB
}

func NewConsumerRepository(db *sql.DB) *ConsumerRepository {
	return &ConsumerRepository{
		db: db,
	}
}

func (repository *ConsumerRepository) Save(consumer consumer.Consumer) error {
	_, err := repository.db.Exec(
		`INSERT INTO consumers (auth0_id, email, name, surname, created_on, updated_on)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		consumer.Auth0ID,
		consumer.Email,
		consumer.Name,
		consumer.Surname,
	)

	return err
}

func (repository *ConsumerRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM consumers`)
	return err
}

func (repository *ConsumerRepository) FindByEmail(email string) bool {
	assert.FailNow(nil, "FindByEmail method is not implemented yet")
	return false
}
