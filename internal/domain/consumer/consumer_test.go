package consumer_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
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
		6,
	)

	require.ErrorIs(t, err, consumer.ErrAddressRequired)
	require.Nil(t, createdConsumer)
}

func TestNewConsumerStoresProvidedValueObjects(t *testing.T) {
	address, err := consumer.NewAddress(" Av. Rivadavia ", " 5100 ", " 4 ", " B ")
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
		6,
	)

	require.NoError(t, err)
	require.Same(t, address, createdConsumer.Address())
	require.Equal(t, &location, createdConsumer.Location())
	require.Equal(t, 6, createdConsumer.CoverageZoneID())
}
