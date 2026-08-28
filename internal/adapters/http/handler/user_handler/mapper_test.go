package user_handler

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/require"
)

func TestCurrentUserResponseIncludesConsumerAddressForItsOwner(t *testing.T) {
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "4", "B")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)
	currentConsumer, err := consumer.NewConsumer(
		"auth0|consumer", "ana@example.com", "Ana", "Perez", nil, address, location,
		coveragezone.CoverageZone{ID: 6, Name: "Comuna 6", Enabled: true},
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

func TestWithIdentityVerificationStatusAddsStatusToProviderResponse(t *testing.T) {
	currentProvider, err := provider.NewProvider(
		"auth0|provider", "juan@example.com", "Juan", "Gomez", &category.Category{ID: 2, Name: "Plomeria"}, nil,
		[]coveragezone.CoverageZone{{ID: 1, Enabled: true}},
	)
	require.NoError(t, err)
	currentProvider.SetPersistenceID(42)
	response, err := currentUserResponseFromDomain(currentProvider, "disconnected")
	require.NoError(t, err)

	response, err = withIdentityVerificationStatus(response, identityverification.StatusInProgress)

	require.NoError(t, err)
	providerResponse, ok := response.(providerCurrentUserResponse)
	require.True(t, ok)
	require.Equal(t, string(identityverification.StatusInProgress), providerResponse.IdentityVerificationStatus)
}

func TestWithIdentityVerificationDetailsAddsApprovalDateToProviderResponse(t *testing.T) {
	currentProvider, err := provider.NewProvider(
		"auth0|provider", "juan@example.com", "Juan", "Gomez", &category.Category{ID: 2, Name: "Plomeria"}, nil,
		[]coveragezone.CoverageZone{{ID: 1, Enabled: true}},
	)
	require.NoError(t, err)
	currentProvider.SetPersistenceID(42)
	response, err := currentUserResponseFromDomain(currentProvider, "disconnected")
	require.NoError(t, err)
	verifiedOn := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	response, err = withIdentityVerificationDetails(response, identityverification.VerificationStatusDetails{
		Status: identityverification.StatusApproved, VerifiedOn: &verifiedOn,
	})

	require.NoError(t, err)
	providerResponse, ok := response.(providerCurrentUserResponse)
	require.True(t, ok)
	require.Equal(t, string(identityverification.StatusApproved), providerResponse.IdentityVerificationStatus)
	require.NotNil(t, providerResponse.IdentityVerifiedOn)
	require.Equal(t, verifiedOn, *providerResponse.IdentityVerifiedOn)
}
