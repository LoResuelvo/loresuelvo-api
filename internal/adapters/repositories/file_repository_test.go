package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanFileRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.Exec("DELETE FROM providers")
	require.NoError(t, err, "could not clean providers")
	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")
	_, err = database.Exec("DELETE FROM files")
	require.NoError(t, err, "could not clean files")
}

func newFileRepositoryTest(t *testing.T) (*repositories.FileRepository, *sql.DB) {
	t.Helper()
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err, "could not connect to test database")
	t.Cleanup(func() {
		cleanFileRepositoryTestDatabase(t, database)
		database.Close()
	})
	cleanFileRepositoryTestDatabase(t, database)
	return repositories.NewFileRepository(database), database
}

func validFileEntity(t *testing.T) filedomain.File {
	t.Helper()
	metadata, err := filedomain.NewFileMetadata("foto.jpg", "image/jpeg", 1024)
	require.NoError(t, err)
	file, err := filedomain.NewPendingFile(
		"4af47f1b-97b6-4b32-baa0-b95d6077f919",
		"files/2026/06/profile_photo/4af47f1b-97b6-4b32-baa0-b95d6077f919.jpg",
		"public-bucket",
		*metadata,
		filedomain.VisibilityPublic,
		filedomain.PurposeProfilePhoto,
		"auth0|provider",
		time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	return *file
}

func TestFileRepositoryCanSaveAndFindFile(t *testing.T) {
	repository, _ := newFileRepositoryTest(t)
	file := validFileEntity(t)

	err := repository.Save(context.Background(), file)
	found, findErr := repository.FindByID(context.Background(), file.ID)

	require.NoError(t, err)
	require.NoError(t, findErr)
	assert.Equal(t, file.ID, found.ID)
	assert.Equal(t, file.Key, found.Key)
	assert.Equal(t, file.Status, found.Status)
	assert.Equal(t, file.UploadedByAuthID, found.UploadedByAuthID)
}

func TestFileRepositoryCanUpdateFileStatus(t *testing.T) {
	repository, _ := newFileRepositoryTest(t)
	file := validFileEntity(t)
	require.NoError(t, repository.Save(context.Background(), file))

	file.Status = filedomain.StatusConfirmed
	err := repository.Save(context.Background(), file)
	found, findErr := repository.FindByID(context.Background(), file.ID)

	require.NoError(t, err)
	require.NoError(t, findErr)
	assert.Equal(t, filedomain.StatusConfirmed, found.Status)
}

func TestFileRepositoryCanFindFilesByIDs(t *testing.T) {
	repository, _ := newFileRepositoryTest(t)
	file := validFileEntity(t)
	require.NoError(t, repository.Save(context.Background(), file))

	found, err := repository.FindByIDs(context.Background(), []string{file.ID, "11111111-1111-4111-8111-111111111111"})

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, file.ID, found[0].ID)
}
