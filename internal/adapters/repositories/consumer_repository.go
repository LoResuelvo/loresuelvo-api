package repositories

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

type ConsumerRepository struct {
	db             *sql.DB
	repositoryUser *UserRepository
}

func NewConsumerRepository(db *sql.DB) *ConsumerRepository {
	return &ConsumerRepository{
		db:             db,
		repositoryUser: NewUserRepository(db),
	}
}

func (repository *ConsumerRepository) Save(consumer consumer.Consumer) error {
	return repository.repositoryUser.Save(*consumer.User)
}

func (repository *ConsumerRepository) FindByEmail(email string) bool {
	return repository.repositoryUser.FindByEmail(email)
}

func (repository *ConsumerRepository) DeleteAll() error {
	return repository.repositoryUser.DeleteAllOf("consumer")
}
