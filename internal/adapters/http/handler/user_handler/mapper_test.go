package user_handler

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/require"
)

func TestCurrentUserResponseIncludesConsumerAddressForItsOwner(t *testing.T) {
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "4", "B")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)
	currentConsumer, err := consumer.NewConsumer(
		"auth0|consumer", "ana@example.com", "Ana", "Perez", nil, address, location, 6,
	)
	require.NoError(t, err)
	currentConsumer.SetPersistenceID(42)

	response, err := currentUserResponseFromDomain(currentConsumer, "disconnected")

	require.NoError(t, err)
	require.Equal(t, consumerCurrentUserResponse{
		currentUserResponse: currentUserResponse{
			ID:                       42,
			Name:                     "Ana",
			Surname:                  "Perez",
			Email:                    "ana@example.com",
			Role:                     consumer.Role,
			CalendarConnectionStatus: "disconnected",
		},
		Address: consumerAddressResponse{
			Street: "Av. Rivadavia", StreetNumber: "5100", Floor: "4", Unit: "B",
			Latitude: -34.6208, Longitude: -58.4386, CoverageZoneID: 6,
		},
	}, response)
}
