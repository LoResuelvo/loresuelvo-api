package repositories

import (
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type ConsumerRepository struct {
	db             *sql.DB
	repositoryUser *UserRepository
}

func NewConsumerRepository(db *sql.DB, repositoryUser *UserRepository) *ConsumerRepository {
	return &ConsumerRepository{
		db:             db,
		repositoryUser: repositoryUser,
	}
}

func (repository *ConsumerRepository) Save(consumer consumer.Consumer) error {
	tx, err := repository.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning consumer transaction: %w", err)
	}

	userID, err := repository.repositoryUser.saveWithTx(tx, *consumer.User)
	if err != nil {
		return rollbackConsumerTx(tx, fmt.Errorf("saving consumer user: %w", err))
	}

	_, err = tx.Exec(
		`INSERT INTO consumers (user_id)
		VALUES ($1)`,
		userID,
	)
	if err != nil {
		return rollbackConsumerTx(tx, fmt.Errorf("saving consumer profile: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing consumer transaction: %w", err)
	}

	return nil
}

func (repository *ConsumerRepository) FindByEmail(email string) bool {
	return repository.repositoryUser.FindByEmail(email)
}

func (repository *ConsumerRepository) FindIDByAuthID(authID string) (int, error) {
	var consumerID int
	err := repository.db.QueryRow(
		`SELECT consumers.id
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE users.auth_id = $1`,
		authID,
	).Scan(&consumerID)
	if err != nil {
		return 0, fmt.Errorf("finding consumer id by auth id: %w", err)
	}

	return consumerID, nil
}

func (repository *ConsumerRepository) FindAuthIDByID(consumerID int) (string, error) {
	var authID string
	err := repository.db.QueryRow(
		`SELECT users.auth_id
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE consumers.id = $1`,
		consumerID,
	).Scan(&authID)
	if err != nil {
		return "", fmt.Errorf("finding consumer auth id by id: %w", err)
	}

	return authID, nil
}

func (repository *ConsumerRepository) FindIDByEmail(email string) (int, error) {
	var consumerID int
	err := repository.db.QueryRow(
		`SELECT consumers.id
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE users.email = $1`,
		email,
	).Scan(&consumerID)
	if err != nil {
		return 0, fmt.Errorf("finding consumer id by email: %w", err)
	}

	return consumerID, nil
}

func (repository *ConsumerRepository) FindByID(consumerID int) (*consumer.Consumer, error) {
	var foundConsumer consumer.Consumer
	var consumerUser user.User
	err := repository.db.QueryRow(
		`SELECT consumers.id,
			users.auth_id,
			users.email,
			users.name,
			users.surname,
			users.role
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE consumers.id = $1`,
		consumerID,
	).Scan(
		&foundConsumer.ID,
		&consumerUser.AuthID,
		&consumerUser.Email,
		&consumerUser.Name,
		&consumerUser.Surname,
		&consumerUser.Role,
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumer by id: %w", err)
	}

	foundConsumer.User = &consumerUser
	return &foundConsumer, nil
}

func (repository *ConsumerRepository) DeleteAll() error {
	return repository.repositoryUser.DeleteAllOf("consumer")
}

func rollbackConsumerTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback consumer transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}
