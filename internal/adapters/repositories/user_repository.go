package repositories

import (
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (repository *UserRepository) Save(userToSave user.User) error {
	_, err := repository.save(userToSave)
	return err
}

func (repository *UserRepository) save(userToSave user.User) (int, error) {
	return saveUser(repository.db, userToSave)
}

func (repository *UserRepository) saveWithTx(tx *sql.Tx, userToSave user.User) (int, error) {
	return saveUser(tx, userToSave)
}

type userQueryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func saveUser(queryRower userQueryRower, userToSave user.User) (int, error) {
	var userID int
	err := queryRower.QueryRow(
		`INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id`,
		userToSave.AuthID,
		userToSave.Email,
		userToSave.Name,
		userToSave.Surname,
		userToSave.Role,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("saving user: %w", err)
	}

	return userID, nil
}

func (repository *UserRepository) FindByEmail(email string) bool {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`,
		email,
	).Scan(&exists)

	if err != nil {
		return false
	}

	return exists
}

func (repository *UserRepository) FindByAuthID(authID string) (*user.User, error) {
	var user user.User
	err := repository.db.QueryRow(
		`SELECT auth_id, email, name, surname, role FROM users WHERE auth_id = $1`,
		authID,
	).Scan(&user.AuthID, &user.Email, &user.Name, &user.Surname, &user.Role)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (repository *UserRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM users`)
	return err
}

func (repository *UserRepository) DeleteAllOf(role string) error {
	_, err := repository.db.Exec(`DELETE FROM users WHERE role = $1`, role)
	return err
}
