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

	_, err = database.Exec("DELETE FROM coverage_markets")
	require.NoError(t, err, "could not clean coverage markets")
}

func savedCoverageMarket(t *testing.T, repository *repositories.CoverageZoneRepository) *coveragezone.Market {
	t.Helper()
	market, err := coveragezone.NewMarket("CABA", "Ciudad Autónoma de Buenos Aires")
	require.NoError(t, err)
	savedMarket, err := repository.SaveMarket(context.Background(), *market)
	require.NoError(t, err)
	return savedMarket
}

func newCoverageZoneRepositoryTest(t *testing.T) *repositories.CoverageZoneRepository {
	t.Helper()

	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
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
	market := savedCoverageMarket(t, repository)
	zone, err := coveragezone.New(market.ID, "CABA-COMMUNE-06", "Comuna 6", coveragezone.KindCommune)
	require.NoError(t, err)

	savedZone, err := repository.Save(context.Background(), *zone)
	require.NoError(t, err)

	foundZone, err := repository.FindByID(context.Background(), savedZone.ID)

	require.NoError(t, err)
	assert.Equal(t, savedZone, foundZone)
}

func TestCoverageZoneRepositoryListsOnlyAvailableZonesInDeterministicOrder(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	market := savedCoverageMarket(t, repository)

	zone14, err := coveragezone.New(market.ID, "CABA-COMMUNE-14", "Comuna 14", coveragezone.KindCommune)
	require.NoError(t, err)
	zone06, err := coveragezone.New(market.ID, "CABA-COMMUNE-06", "Comuna 6", coveragezone.KindCommune)
	require.NoError(t, err)
	disabledZone, err := coveragezone.New(market.ID, "CABA-COMMUNE-15", "Comuna 15", coveragezone.KindCommune)
	require.NoError(t, err)
	disabledZone.Disable()

	savedZone14 := mustSaveCoverageZone(t, repository, zone14)
	savedZone06 := mustSaveCoverageZone(t, repository, zone06)
	mustSaveCoverageZone(t, repository, disabledZone)
	saveCoverageZoneBoundaryReference(t, repository, savedZone14, "google-place-14")
	saveCoverageZoneBoundaryReference(t, repository, savedZone06, "google-place-6")

	zones, err := repository.ListAvailableByMarketCode(context.Background(), " caba ")

	require.NoError(t, err)
	require.Len(t, zones, 2)
	assert.Equal(t, "CABA-COMMUNE-06", zones[0].Zone.Code)
	assert.Equal(t, "google-place-6", zones[0].BoundaryReference.ExternalID)
	assert.Equal(t, "CABA-COMMUNE-14", zones[1].Zone.Code)
	assert.Equal(t, "google-place-14", zones[1].BoundaryReference.ExternalID)
	assert.True(t, zones[0].Zone.Enabled)
	assert.True(t, zones[1].Zone.Enabled)
}

func TestCoverageZoneRepositoryRejectsAvailableZoneWithoutBoundaryReference(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	market := savedCoverageMarket(t, repository)
	zone, err := coveragezone.New(market.ID, "CABA-COMMUNE-06", "Comuna 6", coveragezone.KindCommune)
	require.NoError(t, err)
	mustSaveCoverageZone(t, repository, zone)

	zones, err := repository.ListAvailableByMarketCode(context.Background(), "CABA")

	assert.Nil(t, zones)
	assert.ErrorIs(t, err, coveragezone.ErrBoundaryReferenceNotConfigured)
}

func TestCoverageZoneRepositoryReturnsEmptyListWhenMarketHasNoAvailableZones(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	_ = savedCoverageMarket(t, repository)

	zones, err := repository.ListAvailableByMarketCode(context.Background(), "CABA")

	require.NoError(t, err)
	assert.NotNil(t, zones)
	assert.Empty(t, zones)
}

func TestCoverageZoneRepositoryExcludesZonesFromDisabledMarket(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	market, err := coveragezone.NewMarket("ROSARIO", "Rosario")
	require.NoError(t, err)
	market.Enabled = false
	savedMarket, err := repository.SaveMarket(context.Background(), *market)
	require.NoError(t, err)

	zone, err := coveragezone.New(savedMarket.ID, "ROSARIO-CENTRO", "Centro", coveragezone.KindOperationalZone)
	require.NoError(t, err)
	mustSaveCoverageZone(t, repository, zone)

	zones, err := repository.ListAvailableByMarketCode(context.Background(), "ROSARIO")

	require.NoError(t, err)
	assert.Empty(t, zones)
}

func TestCoverageZoneRepositoryPersistsDomainNormalizedName(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	market := savedCoverageMarket(t, repository)
	zone, err := coveragezone.New(market.ID, "CABA-COMMUNE-06", "  Comuna 6  ", coveragezone.KindCommune)
	require.NoError(t, err)

	savedZone, err := repository.Save(context.Background(), *zone)

	require.NoError(t, err)
	assert.Equal(t, "comuna 6", savedZone.NormalizedName)
}

func TestCoverageZoneRepositoryFindByIDReturnsNotFound(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)

	foundZone, err := repository.FindByID(context.Background(), 999999999)

	assert.Nil(t, foundZone)
	assert.ErrorIs(t, err, coveragezone.ErrDoesNotExist)
}

func TestCoverageZoneRepositoryCanUpdateEnabledState(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	market := savedCoverageMarket(t, repository)
	zone, err := coveragezone.New(market.ID, "CABA-COMMUNE-15", "Comuna 15", coveragezone.KindCommune)
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

func TestCoverageZoneRepositoryScopesNamesByMarket(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	caba := savedCoverageMarket(t, repository)
	rosarioMarket, err := coveragezone.NewMarket("ROSARIO", "Rosario")
	require.NoError(t, err)
	rosario, err := repository.SaveMarket(context.Background(), *rosarioMarket)
	require.NoError(t, err)

	cabaZone, err := coveragezone.New(caba.ID, "CABA-CENTRO", "Centro", coveragezone.KindOperationalZone)
	require.NoError(t, err)
	rosarioZone, err := coveragezone.New(rosario.ID, "ROSARIO-CENTRO", "Centro", coveragezone.KindOperationalZone)
	require.NoError(t, err)
	require.NotNil(t, mustSaveCoverageZone(t, repository, cabaZone))
	require.NotNil(t, mustSaveCoverageZone(t, repository, rosarioZone))

	foundCABA, err := repository.FindByMarketCodeAndName(context.Background(), "CABA", "Centro")
	require.NoError(t, err)
	foundRosario, err := repository.FindByMarketCodeAndName(context.Background(), "ROSARIO", "Centro")
	require.NoError(t, err)

	assert.Equal(t, caba.ID, foundCABA.MarketID)
	assert.Equal(t, rosario.ID, foundRosario.MarketID)
}

func TestCoverageZoneRepositoryResolvesGenericExternalReferences(t *testing.T) {
	repository := newCoverageZoneRepositoryTest(t)
	market := savedCoverageMarket(t, repository)
	zone, err := coveragezone.New(market.ID, "CABA-COMMUNE-06", "Comuna 6", coveragezone.KindCommune)
	require.NoError(t, err)
	savedZone := mustSaveCoverageZone(t, repository, zone)
	reference, err := coveragezone.NewExternalReference(savedZone.ID, "google", "google-place-6", "2026-08")
	require.NoError(t, err)

	require.NoError(t, repository.SaveExternalReference(context.Background(), *reference))
	foundZone, err := repository.FindByExternalReference(context.Background(), "GOOGLE", "google-place-6")

	require.NoError(t, err)
	assert.Equal(t, savedZone, foundZone)
}

func mustSaveCoverageZone(t *testing.T, repository *repositories.CoverageZoneRepository, zone *coveragezone.CoverageZone) *coveragezone.CoverageZone {
	t.Helper()
	savedZone, err := repository.Save(context.Background(), *zone)
	require.NoError(t, err)
	return savedZone
}

func saveCoverageZoneBoundaryReference(
	t *testing.T,
	repository *repositories.CoverageZoneRepository,
	zone *coveragezone.CoverageZone,
	externalID string,
) {
	t.Helper()
	reference, err := coveragezone.NewExternalReference(zone.ID, "GOOGLE", externalID, "repository-test")
	require.NoError(t, err)
	require.NoError(t, repository.SaveExternalReference(context.Background(), *reference))
}
