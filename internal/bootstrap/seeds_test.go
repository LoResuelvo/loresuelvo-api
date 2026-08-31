package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expectedGoogleCommunePlaceIDs = []string{
	"ChIJuQQBxzE1o5URe4mzo2cEmTc",
	"ChIJDWsSDp_KvJUR9QfscYcpGOU",
	"ChIJfYHTguXKvJURoZj4BULLfqM",
	"ChIJ9_j8amvLvJURXE6r-8YAdB8",
	"ChIJA7bs4VnKvJUR8YI31twWxkg",
	"ChIJIR9pFD7KvJURPaMYFhbLY48",
	"ChIJs7AojjLKvJURJNFcnyOteb0",
	"ChIJmZurhlXJvJUR4nMVILqKWx4",
	"ChIJK4_D6QjJvJURJsjRJpB3mVg",
	"ChIJif2PN8nJvJURdmM8-GjGdMs",
	"ChIJ9fAAUSS2vJUR1sOIFvvX2XU",
	"ChIJX3MKLfS2vJURrrZUmIOQI-0",
	"ChIJ1xHQhS20vJURuGiIQEy6qLk",
	"ChIJy9bjmp61vJUR1kFz4gnyucs",
	"ChIJJ2FHGQi2vJURwDLt3Or-hBA",
}

func TestProviderSeedFilesDefineTheCompleteCoverageCatalog(t *testing.T) {
	for _, path := range []string{"../../seeds/default.yaml", "../../seeds/providers-100.yaml"} {
		t.Run(path, func(t *testing.T) {
			data, found, err := loadSeedData(path)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, []coverageMarketSeed{{Code: "CABA", Name: "Ciudad Autónoma de Buenos Aires", Enabled: true}}, data.CoverageMarkets)
			require.Len(t, data.CoverageZones, 15)

			enabledZoneCodes := make(map[string]struct{}, 15)
			googlePlaceIDs := make(map[string]struct{}, 15)
			for index, zone := range data.CoverageZones {
				require.True(t, zone.Enabled, "coverage zone %q must be enabled", zone.Name)
				require.Equal(t, "CABA", zone.MarketCode)
				require.Equal(t, "COMMUNE", zone.Kind)
				require.Len(t, zone.ExternalReferences, 1)
				reference := zone.ExternalReferences[0]
				require.Equal(t, "GOOGLE", reference.Provider)
				require.Equal(t, expectedGoogleCommunePlaceIDs[index], reference.ExternalID)
				require.Equal(t, "region-lookup-v1alpha@2026-08-21", reference.SourceVersion)
				googlePlaceIDs[reference.ExternalID] = struct{}{}
				enabledZoneCodes[strings.ToUpper(strings.TrimSpace(zone.Code))] = struct{}{}
			}
			require.Len(t, enabledZoneCodes, 15, "coverage zone codes must be unique")
			require.Len(t, googlePlaceIDs, 15, "Google Place IDs must be unique")
			for commune := 1; commune <= 15; commune++ {
				_, exists := enabledZoneCodes[fmt.Sprintf("CABA-COMMUNE-%02d", commune)]
				assert.True(t, exists, "Comuna %d must be seeded", commune)
			}

			for _, provider := range data.Providers {
				require.NotEmpty(t, provider.CoverageZoneCodes, "seed provider %q must have coverage", provider.AuthID)
				for _, zoneCode := range provider.CoverageZoneCodes {
					_, exists := enabledZoneCodes[strings.ToUpper(strings.TrimSpace(zoneCode))]
					assert.True(t, exists, "seed provider %q references unknown coverage zone %q", provider.AuthID, zoneCode)
				}
			}
		})
	}
}

func TestSeedDefaultDataSeedsCoverageCatalogAndProviderAssociationsIdempotently(t *testing.T) {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	seed := providerCoverageSeedData("auth0|coverage-seed", "coverage-seed@example.com", []string{"CABA-COMMUNE-06", "CABA-COMMUNE-14"})
	cleanupProviderCoverageSeed(t, database, seed)
	t.Cleanup(func() { cleanupProviderCoverageSeed(t, database, seed) })

	require.NoError(t, seedDefaultData(context.Background(), database, "test-public-bucket", seed))
	seed.Providers[0].CoverageZoneCodes = []string{"CABA-COMMUNE-01", "CABA-COMMUNE-13"}
	require.NoError(t, seedDefaultData(context.Background(), database, "test-public-bucket", seed))
	require.NoError(t, seedDefaultData(context.Background(), database, "test-public-bucket", seed))

	var catalogCount int
	err = database.QueryRow(`SELECT COUNT(*) FROM coverage_zones WHERE normalized_name LIKE 'comuna %'`).Scan(&catalogCount)
	require.NoError(t, err)
	assert.Equal(t, 15, catalogCount)

	var externalReferenceCount int
	require.NoError(t, database.QueryRow(
		`SELECT COUNT(*)
		FROM coverage_zone_external_references
		WHERE provider = 'GOOGLE' AND source_version = 'region-lookup-v1alpha@2026-08-21'`,
	).Scan(&externalReferenceCount))
	assert.Equal(t, 15, externalReferenceCount)

	rows, err := database.Query(
		`SELECT coverage_zones.name
		FROM provider_coverage_zones
		INNER JOIN coverage_zones ON coverage_zones.id = provider_coverage_zones.coverage_zone_id
		INNER JOIN users ON users.id = provider_coverage_zones.provider_id
		WHERE users.auth_id = $1
		ORDER BY coverage_zones.normalized_name`,
		seed.Providers[0].AuthID,
	)
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"Comuna 1", "Comuna 13"}, names)
}

func TestSeedDefaultDataRollsBackWhenProviderCoverageZoneDoesNotExist(t *testing.T) {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	seed := providerCoverageSeedData("auth0|invalid-coverage-seed", "invalid-coverage-seed@example.com", []string{"CABA-COMMUNE-99"})
	cleanupProviderCoverageSeed(t, database, seed)
	t.Cleanup(func() { cleanupProviderCoverageSeed(t, database, seed) })

	err = seedDefaultData(context.Background(), database, "test-public-bucket", seed)

	require.Error(t, err)
	assert.ErrorContains(t, err, `coverage zone "CABA-COMMUNE-99"`)
	var userCount int
	require.NoError(t, database.QueryRow("SELECT COUNT(*) FROM users WHERE auth_id = $1", seed.Providers[0].AuthID).Scan(&userCount))
	assert.Zero(t, userCount)
}

func providerCoverageSeedData(authID, email string, providerCoverageZones []string) seedData {
	zones := make([]coverageZoneSeed, 0, 15)
	for commune := 1; commune <= 15; commune++ {
		zones = append(zones, coverageZoneSeed{
			MarketCode: "CABA",
			Code:       fmt.Sprintf("CABA-COMMUNE-%02d", commune),
			Name:       fmt.Sprintf("Comuna %d", commune),
			Kind:       "COMMUNE",
			Enabled:    true,
			ExternalReferences: []coverageZoneExternalReferenceSeed{{
				Provider:      "GOOGLE",
				ExternalID:    expectedGoogleCommunePlaceIDs[commune-1],
				SourceVersion: "region-lookup-v1alpha@2026-08-21",
			}},
		})
	}

	return seedData{
		CoverageMarkets: []coverageMarketSeed{{Code: "CABA", Name: "Ciudad Autónoma de Buenos Aires", Enabled: true}},
		CoverageZones:   zones,
		Categories:      []categorySeed{{Name: "Plomería"}},
		Providers: []providerSeed{{
			AuthID:             authID,
			Email:              email,
			Name:               "Seed",
			Surname:            "Provider",
			CategoryName:       "Plomería",
			CoverageZoneCodes:  providerCoverageZones,
			ProfilePhotoFileID: "33333333-3333-4333-8333-333333333333",
			ProfilePhotoName:   "coverage-seed.webp",
			ProfilePhotoKey:    "seed/profile_photo/coverage-seed.webp",
		}},
	}
}

func cleanupProviderCoverageSeed(t *testing.T, database *sql.DB, seed seedData) {
	t.Helper()

	_, err := database.Exec("DELETE FROM users WHERE auth_id = $1", seed.Providers[0].AuthID)
	require.NoError(t, err)
	_, err = database.Exec("DELETE FROM files WHERE id = $1", seed.Providers[0].ProfilePhotoFileID)
	require.NoError(t, err)
	_, err = database.Exec(`DELETE FROM provider_coverage_zones
		WHERE coverage_zone_id IN (
			SELECT coverage_zones.id
			FROM coverage_zones
			INNER JOIN coverage_markets ON coverage_markets.id = coverage_zones.market_id
			WHERE coverage_markets.code = 'CABA'
		)`)
	require.NoError(t, err)
	_, err = database.Exec(`DELETE FROM consumer_addresses
		WHERE coverage_zone_id IN (
			SELECT coverage_zones.id
			FROM coverage_zones
			INNER JOIN coverage_markets ON coverage_markets.id = coverage_zones.market_id
			WHERE coverage_markets.code = 'CABA'
		)`)
	require.NoError(t, err)
	_, err = database.Exec(`DELETE FROM coverage_zones
		WHERE market_id IN (SELECT id FROM coverage_markets WHERE code = 'CABA')`)
	require.NoError(t, err)
	_, err = database.Exec("DELETE FROM coverage_markets WHERE code = 'CABA'")
	require.NoError(t, err)
}
