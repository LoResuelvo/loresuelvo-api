package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanCoverageZoneRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM provider_coverage_zones")
	require.NoError(t, err, "could not clean provider coverage zones")

	_, err = database.Exec("DELETE FROM coverage_zones")
	require.NoError(t, err, "could not clean coverage zones")
}

func newCoverageZoneRepositoryTest(t *testing.T) *repositories.CoverageZoneRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()
	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanCoverageZoneRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanCoverageZoneRepositoryTestDatabase(t, database)

	return repositories.NewCoverageZoneRepository(database)
}

func TestCoverageZoneRepositoryCanFindByID(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	zone, err := coveragezone.New("Comuna 6")
	require.NoError(t, err)

	savedZone, err := repository.Save(context.Background(), *zone)
	require.NoError(t, err)

	foundZone, err := repository.FindByID(context.Background(), savedZone.ID)

	require.NoError(t, err)
	assert.Equal(t, savedZone, foundZone)
}

func TestCoverageZoneRepositoryFindByIDReturnsNotFound(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)

	foundZone, err := repository.FindByID(context.Background(), 999999999)

	assert.Nil(t, foundZone)
	assert.ErrorIs(t, err, coveragezone.ErrDoesNotExist)
}

func TestCoverageZoneRepositoryCanUpdateEnabledState(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	zone, err := coveragezone.New("Comuna 15")
	require.NoError(t, err)

	savedZone, err := repository.Save(context.Background(), *zone)
	require.NoError(t, err)
	savedZone.Disable()

	err = repository.Update(context.Background(), *savedZone)

	require.NoError(t, err)
	foundZone, err := repository.FindByID(context.Background(), savedZone.ID)
	require.NoError(t, err)
	assert.False(t, foundZone.Enabled)
}
