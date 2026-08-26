package consumer_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/stretchr/testify/require"
)

func TestNewConsumerRequiresAddress(t *testing.T) {
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)

	createdConsumer, err := consumer.NewConsumer(
		"auth0|consumer",
		"consumer@example.com",
		"Ana",
		"Perez",
		nil,
		nil,
		location,
		coveragezone.CoverageZone{ID: 6, Name: "Comuna 6", Enabled: true},
	)

	require.ErrorIs(t, err, consumer.ErrAddressRequired)
	require.Nil(t, createdConsumer)
}

func TestNewConsumerStoresProvidedValueObjects(t *testing.T) {
	address, err := consumer.NewAddress(" Av. Rivadavia ", " 5100 ", " 4 ", " B ")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)
	coverageZone := coveragezone.CoverageZone{ID: 6, Name: "Comuna 6", Enabled: true}

	createdConsumer, err := consumer.NewConsumer(
		"auth0|consumer",
		"consumer@example.com",
		"Ana",
		"Perez",
		nil,
		address,
		location,
		coverageZone,
	)

	require.NoError(t, err)
	require.Equal(t, *address, createdConsumer.Address())
	require.Equal(t, location, createdConsumer.Location())
	require.Equal(t, coverageZone, createdConsumer.CoverageZone())
}

func TestNewConsumerRequiresCoverageZone(t *testing.T) {
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "", "")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)

	createdConsumer, err := consumer.NewConsumer(
		"auth0|consumer",
		"consumer@example.com",
		"Ana",
		"Perez",
		nil,
		address,
		location,
		coveragezone.CoverageZone{},
	)

	require.ErrorIs(t, err, consumer.ErrCoverageZoneNotAvailable)
	require.Nil(t, createdConsumer)
}

func TestNewConsumerRejectsDisabledCoverageZone(t *testing.T) {
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "", "")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)

	createdConsumer, err := consumer.NewConsumer(
		"auth0|consumer",
		"consumer@example.com",
		"Ana",
		"Perez",
		nil,
		address,
		location,
		coveragezone.CoverageZone{ID: 6, Name: "Comuna 6"},
	)

	require.ErrorIs(t, err, consumer.ErrCoverageZoneNotAvailable)
	require.Nil(t, createdConsumer)
}
