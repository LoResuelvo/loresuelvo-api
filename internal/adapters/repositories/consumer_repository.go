package repositories

import (
	"context"
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

func (repository *ConsumerRepository) DeleteAll() error {
	return repository.repositoryUser.DeleteAllOf("consumer")
}

func (repository *ConsumerRepository) FindByIDs(ctx context.Context, ids []int) ([]consumer.Consumer, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT
			consumers.id,
			users.auth_id,
			users.email,
			users.name,
			users.surname,
			users.role
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE consumers.id = ANY($1)
		ORDER BY users.name ASC, users.surname ASC`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumers by ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []consumer.Consumer
	for rows.Next() {
		var cons consumer.Consumer
		var usr user.User

		if err := rows.Scan(&cons.ID, &usr.AuthID, &usr.Email, &usr.Name, &usr.Surname, &usr.Role); err != nil {
			return nil, fmt.Errorf("scanning consumer: %w", err)
		}

		cons.User = &usr
		results = append(results, cons)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating consumers: %w", err)
	}

	return results, nil
}

func rollbackConsumerTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback consumer transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}
