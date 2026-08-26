package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/require"
)

func consumerWithAddress(t *testing.T, database *sql.DB, authID, email, name, surname string) *consumer.Consumer {
	t.Helper()

	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "", "")
	if err != nil {
		panic(err)
	}

	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	if err != nil {
		panic(err)
	}
	coverageZoneID := consumerTestCoverageZoneID(t, database)
	coverageZone, err := repositories.NewCoverageZoneRepository(database).FindByID(context.Background(), coverageZoneID)
	if err != nil {
		panic(err)
	}

	createdConsumer, err := consumer.NewConsumer(
		authID,
		email,
		name,
		surname,
		nil,
		address,
		location,
		*coverageZone,
	)
	if err != nil {
		panic(err)
	}

	return createdConsumer
}

func consumerTestCoverageZoneID(t *testing.T, database *sql.DB) int {
	t.Helper()

	var coverageZoneID int
	err := database.QueryRowContext(
		context.Background(),
		`SELECT coverage_zones.id
		FROM coverage_zones
		JOIN coverage_markets ON coverage_markets.id = coverage_zones.market_id
		WHERE coverage_zones.enabled = TRUE AND coverage_markets.enabled = TRUE
		ORDER BY coverage_zones.id
		LIMIT 1`,
	).Scan(&coverageZoneID)
	if err == nil {
		return coverageZoneID
	}
	if !errors.Is(err, sql.ErrNoRows) {
		require.NoError(t, err, "could not find consumer test coverage zone")
	}

	var marketID int
	err = database.QueryRowContext(
		context.Background(),
		`INSERT INTO coverage_markets (code, name, enabled, created_on, updated_on)
		VALUES ('CABA-CONSUMER-FIXTURE', 'Consumer fixture market', TRUE, NOW(), NOW())
		ON CONFLICT (code) DO UPDATE SET enabled = TRUE, updated_on = NOW()
		RETURNING id`,
	).Scan(&marketID)
	require.NoError(t, err, "could not create consumer test coverage market")

	err = database.QueryRowContext(
		context.Background(),
		`INSERT INTO coverage_zones (market_id, code, name, normalized_name, kind, enabled, created_on, updated_on)
		VALUES ($1, 'CABA-CONSUMER-FIXTURE-01', 'Consumer Fixture Zone', 'consumer fixture zone', 'COMMUNE', TRUE, NOW(), NOW())
		ON CONFLICT (code) DO UPDATE SET enabled = TRUE, updated_on = NOW()
		RETURNING id`,
		marketID,
	).Scan(&coverageZoneID)
	require.NoError(t, err, "could not create consumer test coverage zone")

	return coverageZoneID
}
