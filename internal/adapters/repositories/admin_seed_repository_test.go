package repositories_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSeedAuthID  = "test-admin-seed-identity"
	testSeedEmail   = "admin-seed@example.invalid"
	testSeedName    = "Seed"
	testSeedSurname = "Administrator"
)

type persistedAdminSeed struct {
	authID             string
	email              string
	name               string
	surname            string
	role               string
	profilePhotoFileID sql.NullString
	createdOn          time.Time
	updatedOn          time.Time
}

func TestEnsureAdminCreatesSeedIdempotentlyWithoutUpdatingTimestamps(t *testing.T) {
	repository, database := newAdminSeedRepositoryTest(t)
	expected := newSeedAdmin(t, testSeedAuthID, testSeedEmail, testSeedName, testSeedSurname)

	require.NoError(t, repository.EnsureAdmin(context.Background(), expected))
	require.Positive(t, expected.ID())
	_, err := database.Exec(
		`UPDATE users SET created_on = TIMESTAMP '2000-01-01 00:00:00', updated_on = TIMESTAMP '2000-01-02 00:00:00'
		WHERE auth_id = $1`,
		testSeedAuthID,
	)
	require.NoError(t, err)

	repeated := newSeedAdmin(t, testSeedAuthID, testSeedEmail, testSeedName, testSeedSurname)
	require.NoError(t, repository.EnsureAdmin(context.Background(), repeated))

	assert.Equal(t, expected.ID(), repeated.ID())
	persisted := findPersistedAdminSeed(t, database, testSeedAuthID)
	assert.Equal(t, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), persisted.createdOn.UTC())
	assert.Equal(t, time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC), persisted.updatedOn.UTC())
	assert.False(t, persisted.profilePhotoFileID.Valid)
}

func TestEnsureAdminRejectsConflictsWithoutModifyingExistingUser(t *testing.T) {
	tests := []struct {
		name          string
		existing      persistedAdminSeed
		configured    persistedAdminSeed
		profileFileID string
	}{
		{
			name:       "auth ID belongs to another profile",
			existing:   persistedAdminSeed{authID: testSeedAuthID, email: "other@example.invalid", name: testSeedName, surname: testSeedSurname, role: admin.Role},
			configured: persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname},
		},
		{
			name:       "email belongs to another identity",
			existing:   persistedAdminSeed{authID: "other-admin-seed-identity", email: testSeedEmail, name: testSeedName, surname: testSeedSurname, role: admin.Role},
			configured: persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname},
		},
		{
			name:       "name differs",
			existing:   persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: "Different", surname: testSeedSurname, role: admin.Role},
			configured: persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname},
		},
		{
			name:       "surname differs",
			existing:   persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: "Different", role: admin.Role},
			configured: persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname},
		},
		{
			name:       "role differs",
			existing:   persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname, role: "provider"},
			configured: persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository, database := newAdminSeedRepositoryTest(t)
			insertPersistedAdminSeed(t, database, test.existing, test.profileFileID)
			before := findPersistedAdminSeed(t, database, test.existing.authID)
			configured := newSeedAdmin(t, test.configured.authID, test.configured.email, test.configured.name, test.configured.surname)

			err := repository.EnsureAdmin(context.Background(), configured)

			require.ErrorIs(t, err, repositories.ErrAdminSeedConflict)
			assert.Equal(t, before, findPersistedAdminSeed(t, database, test.existing.authID))
		})
	}
}

func TestEnsureAdminPreservesExistingProfilePhoto(t *testing.T) {
	repository, database := newAdminSeedRepositoryTest(t)
	existing := persistedAdminSeed{authID: testSeedAuthID, email: testSeedEmail, name: testSeedName, surname: testSeedSurname, role: admin.Role}
	insertPersistedAdminSeed(t, database, existing, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

	err := repository.EnsureAdmin(
		context.Background(),
		newSeedAdmin(t, testSeedAuthID, testSeedEmail, testSeedName, testSeedSurname),
	)

	require.NoError(t, err)
	persisted := findPersistedAdminSeed(t, database, testSeedAuthID)
	require.True(t, persisted.profilePhotoFileID.Valid)
	assert.Equal(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", persisted.profilePhotoFileID.String)
}

func TestEnsureAdminIsIdempotentAcrossConcurrentAttempts(t *testing.T) {
	repository, database := newAdminSeedRepositoryTest(t)
	const attempts = 8
	start := make(chan struct{})
	errorsByAttempt := make(chan error, attempts)
	configuredAdmins := make([]*admin.Admin, attempts)
	for index := range attempts {
		configuredAdmins[index] = newSeedAdmin(t, testSeedAuthID, testSeedEmail, testSeedName, testSeedSurname)
	}
	var waitGroup sync.WaitGroup
	for index := range attempts {
		waitGroup.Add(1)
		go func(configured *admin.Admin) {
			defer waitGroup.Done()
			<-start
			errorsByAttempt <- repository.EnsureAdmin(context.Background(), configured)
		}(configuredAdmins[index])
	}
	close(start)
	waitGroup.Wait()
	close(errorsByAttempt)

	for err := range errorsByAttempt {
		assert.NoError(t, err)
	}
	var count int
	require.NoError(t, database.QueryRow(
		"SELECT COUNT(*) FROM users WHERE auth_id = $1 OR email = $2",
		testSeedAuthID,
		testSeedEmail,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestEnsureAdminPreservesContextCancellationAndSanitizesPersistenceErrors(t *testing.T) {
	repository, _ := newAdminSeedRepositoryTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repository.EnsureAdmin(ctx, newSeedAdmin(t, testSeedAuthID, testSeedEmail, testSeedName, testSeedSurname))

	require.ErrorIs(t, err, repositories.ErrAdminSeedPersistence)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), testSeedAuthID)
	assert.NotContains(t, err.Error(), testSeedEmail)
}

func newAdminSeedRepositoryTest(t *testing.T) (*repositories.UserRepository, *sql.DB) {
	t.Helper()
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := database.Exec("DELETE FROM users WHERE auth_id IN ($1, $2) OR email IN ($3, $4)", testSeedAuthID, "other-admin-seed-identity", testSeedEmail, "other@example.invalid")
		assert.NoError(t, cleanupErr)
		_, cleanupErr = database.Exec("DELETE FROM files WHERE id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'")
		assert.NoError(t, cleanupErr)
		assert.NoError(t, database.Close())
	})
	_, err = database.Exec("DELETE FROM users WHERE auth_id IN ($1, $2) OR email IN ($3, $4)", testSeedAuthID, "other-admin-seed-identity", testSeedEmail, "other@example.invalid")
	require.NoError(t, err)
	_, err = database.Exec("DELETE FROM files WHERE id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'")
	require.NoError(t, err)
	return repositories.NewUserRepository(database), database
}

func newSeedAdmin(t *testing.T, authID, email, name, surname string) *admin.Admin {
	t.Helper()
	configured, err := admin.NewAdmin(authID, email, name, surname, nil)
	require.NoError(t, err)
	return configured
}

func insertPersistedAdminSeed(t *testing.T, database *sql.DB, seed persistedAdminSeed, profileFileID string) {
	t.Helper()
	var nullableProfileFileID any
	if profileFileID != "" {
		_, err := database.Exec(
			`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
			VALUES ($1, $2, 'test', 'profile.webp', 'image/webp', 1, 'confirmed', 'private', 'profile_photo', $3, NOW(), NOW())`,
			profileFileID,
			"admin-seed/"+profileFileID,
			seed.authID,
		)
		require.NoError(t, err)
		nullableProfileFileID = profileFileID
	}
	_, err := database.Exec(
		`INSERT INTO users (auth_id, email, name, surname, role, profile_photo_file_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, TIMESTAMP '2000-01-01 00:00:00', TIMESTAMP '2000-01-02 00:00:00')`,
		seed.authID,
		seed.email,
		seed.name,
		seed.surname,
		seed.role,
		nullableProfileFileID,
	)
	require.NoError(t, err)
}

func findPersistedAdminSeed(t *testing.T, database *sql.DB, authID string) persistedAdminSeed {
	t.Helper()
	var seed persistedAdminSeed
	err := database.QueryRow(
		`SELECT auth_id, email, name, surname, role, profile_photo_file_id, created_on, updated_on
		FROM users WHERE auth_id = $1`,
		authID,
	).Scan(
		&seed.authID,
		&seed.email,
		&seed.name,
		&seed.surname,
		&seed.role,
		&seed.profilePhotoFileID,
		&seed.createdOn,
		&seed.updatedOn,
	)
	require.NoError(t, err)
	return seed
}
