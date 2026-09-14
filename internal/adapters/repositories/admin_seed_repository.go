package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
)

var (
	ErrAdminSeedConflict    = errors.New("admin seed conflicts with an existing user")
	ErrAdminSeedPersistence = errors.New("admin seed could not be persisted")
)

// EnsureAdmin creates the configured administrator or verifies that the
// persisted profile is an exact match. Existing users are never modified.
func (repository *UserRepository) EnsureAdmin(ctx context.Context, expected *admin.Admin) error {
	if expected == nil || expected.Role() != admin.Role {
		return ErrAdminSeedConflict
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return adminSeedPersistenceError(ctx)
	}
	defer func() { _ = tx.Rollback() }()

	var userID int
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO users (auth_id, email, name, surname, role, profile_photo_file_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, NULL, NOW(), NOW())
		ON CONFLICT DO NOTHING
		RETURNING id`,
		expected.AuthID(),
		expected.Email(),
		expected.Name(),
		expected.Surname(),
		expected.Role(),
	).Scan(&userID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return adminSeedPersistenceError(ctx)
		}
		expected.SetPersistenceID(userID)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return adminSeedPersistenceError(ctx)
	}

	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, auth_id, email, name, surname, role
		FROM users
		WHERE auth_id = $1 OR email = $2`,
		expected.AuthID(),
		expected.Email(),
	)
	if err != nil {
		return adminSeedPersistenceError(ctx)
	}

	matchCount := 0
	for rows.Next() {
		var id int
		var authID, email, name, surname, role string
		if err := rows.Scan(&id, &authID, &email, &name, &surname, &role); err != nil {
			_ = rows.Close()
			return adminSeedPersistenceError(ctx)
		}
		if authID != expected.AuthID() ||
			email != expected.Email() ||
			name != expected.Name() ||
			surname != expected.Surname() ||
			role != admin.Role {
			_ = rows.Close()
			return ErrAdminSeedConflict
		}
		matchCount++
		userID = id
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return adminSeedPersistenceError(ctx)
	}
	if err := rows.Close(); err != nil {
		return adminSeedPersistenceError(ctx)
	}
	if matchCount != 1 {
		return ErrAdminSeedConflict
	}

	if err := tx.Commit(); err != nil {
		return adminSeedPersistenceError(ctx)
	}
	expected.SetPersistenceID(userID)
	return nil
}

func adminSeedPersistenceError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrAdminSeedPersistence, err)
	}
	return ErrAdminSeedPersistence
}
