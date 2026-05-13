package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
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
		`INSERT INTO consumers (email, name, surname, password, created_on, updated_on)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		consumer.Email,
		consumer.Name,
		consumer.Surname,
		consumer.Password,
	)

	return err
}

func (repository *ConsumerRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM consumers`)
	return err
}
