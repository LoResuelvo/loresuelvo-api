package admin_handler

import (
	"testing"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsumerDirectoryResponsesMapPublicContract(t *testing.T) {
	createdOn := time.Date(2026, 9, 19, 12, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	responses := consumerDirectoryResponsesFromReadModel([]readmodel.Consumer{
		{
			ID:              10,
			Name:            "Ana",
			Surname:         "Pérez",
			Email:           "ana@example.com",
			ProfilePhotoURL: "https://cdn.example/profile.jpg",
			CreatedOn:       createdOn,
		},
		{
			ID:        11,
			Name:      "Beatriz",
			Surname:   "Suárez",
			Email:     "beatriz@example.com",
			CreatedOn: createdOn,
		},
	})

	require.Len(t, responses, 2)
	assert.Equal(t, consumerDirectoryResponse{
		ID:              10,
		Role:            consumer.Role,
		Name:            "Ana",
		Surname:         "Pérez",
		Email:           "ana@example.com",
		ProfilePhotoURL: stringPointer("https://cdn.example/profile.jpg"),
		CreatedOn:       createdOn.UTC(),
	}, responses[0])
	assert.Nil(t, responses[1].ProfilePhotoURL)
}

func TestConsumerDirectoryResponsesReturnNonNilEmptySlice(t *testing.T) {
	responses := consumerDirectoryResponsesFromReadModel(nil)

	assert.NotNil(t, responses)
	assert.Empty(t, responses)
}

func TestProviderDirectoryResponsesMapPublicContract(t *testing.T) {
	createdOn := time.Date(2026, 9, 19, 12, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	verifiedOn := time.Date(2026, 9, 18, 15, 30, 0, 0, time.UTC)
	responses := providerDirectoryResponsesFromReadModel([]readmodel.Provider{{
		ID:                         20,
		Name:                       "Juan",
		Surname:                    "Gómez",
		Email:                      "juan@example.com",
		ProfilePhotoURL:            "https://cdn.example/profile.jpg",
		CreatedOn:                  createdOn,
		Category:                   readmodel.ProviderCategory{ID: 2, Name: "Plomería"},
		CoverageZones:              []readmodel.ProviderCoverageZone{{ID: 6, MarketID: 1, Code: "CABA-COMMUNE-06", Name: "Comuna 6", Kind: coveragezone.KindCommune, Enabled: true}},
		IdentityVerificationStatus: identityverification.StatusApproved,
		IdentityVerifiedOn:         &verifiedOn,
	}})

	require.Len(t, responses, 1)
	assert.Equal(t, providerDirectoryResponse{
		ID:              20,
		Role:            provider.Role,
		Name:            "Juan",
		Surname:         "Gómez",
		Email:           "juan@example.com",
		ProfilePhotoURL: stringPointer("https://cdn.example/profile.jpg"),
		CreatedOn:       createdOn.UTC(),
		Category:        providerDirectoryCategory{ID: 2, Name: "Plomería"},
		CoverageZones: []providerDirectoryCoverageZone{{
			ID: 6, MarketID: 1, Code: "CABA-COMMUNE-06", Name: "Comuna 6", Kind: string(coveragezone.KindCommune), Enabled: true,
		}},
		IdentityVerificationStatus: string(identityverification.StatusApproved),
		IdentityVerifiedOn:         &verifiedOn,
	}, responses[0])
}

func TestProviderDirectoryResponsesReturnNonNilEmptySlice(t *testing.T) {
	responses := providerDirectoryResponsesFromReadModel(nil)

	assert.NotNil(t, responses)
	assert.Empty(t, responses)
}

func stringPointer(value string) *string {
	return &value
}
