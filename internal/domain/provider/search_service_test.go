package provider_test

import (
	"context"
	"errors"
	"testing"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceComposesRatingSummaryForEachProviderSearchResult(t *testing.T) {
	providerCategory := existingCategory()
	juan, err := provider.NewProvider(
		"auth0|juan",
		"juan@example.com",
		"Juan",
		"Pérez",
		&providerCategory,
		&filedomain.Image{FileID: "juan-photo", URL: "https://cdn.example/juan.jpg"},
		[]coveragezone.CoverageZone{defaultCoverageZone()},
	)
	require.NoError(t, err)
	juan.SetPersistenceID(12)

	pedro, err := provider.NewProvider(
		"auth0|pedro",
		"pedro@example.com",
		"Pedro",
		"Dib",
		&providerCategory,
		&filedomain.Image{FileID: "pedro-photo", URL: "https://cdn.example/pedro.jpg"},
		[]coveragezone.CoverageZone{defaultCoverageZone()},
	)
	require.NoError(t, err)
	pedro.SetPersistenceID(15)

	profileReader := &providerProfileReaderMock{
		statsByProviderID: map[int]provider.RatingStats{
			juan.ID():  {Total: 9, Count: 2},
			pedro.ID(): {Total: 2, Count: 1},
		},
	}
	identityApprovalReader := &identityApprovalReaderMock{approvedByProviderID: map[int]bool{
		juan.ID(): true,
	}}
	repository := &providerRepositoryMock{
		providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*juan, *pedro},
		},
	}
	providerService := provider.NewService(
		repository,
		categoryFinderWithExistingCategory(),
		&profilePhotoValidatorMock{
			profilePhotoURLsByFile: map[string]string{
				"juan-photo":  "https://cdn.example/juan.jpg",
				"pedro-photo": "https://cdn.example/pedro.jpg",
			},
		},
		profileReader,
		nil,
		identityApprovalReader,
	)

	results, err := providerService.SearchProvidersByCategoryID(context.Background(), providerCategory.ID)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, []int{juan.ID(), pedro.ID()}, profileReader.providerIDs)
	assert.Equal(t, []int{juan.ID(), pedro.ID()}, identityApprovalReader.providerIDs)
	assert.Equal(t, readmodel.ProviderSearchResult{
		ID:               juan.ID(),
		Name:             "Juan",
		Surname:          "Pérez",
		CategoryName:     "Plomería",
		ProfilePhoto:     juan.ProfilePhoto(),
		RatingAverage:    4.5,
		RatingCount:      2,
		IdentityVerified: true,
	}, results[0])
	assert.Equal(t, 2.0, results[1].RatingAverage)
	assert.Equal(t, 1, results[1].RatingCount)
}

func TestServiceUsesZeroRatingSummaryWhenProviderHasNoRatings(t *testing.T) {
	providerCategory := existingCategory()
	foundProvider, err := provider.NewProvider(
		"auth0|juan",
		"juan@example.com",
		"Juan",
		"Pérez",
		&providerCategory,
		&filedomain.Image{FileID: "juan-photo"},
		[]coveragezone.CoverageZone{defaultCoverageZone()},
	)
	require.NoError(t, err)
	foundProvider.SetPersistenceID(12)
	profileReader := &providerProfileReaderMock{statsByProviderID: map[int]provider.RatingStats{}}
	providerService := provider.NewService(
		&providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*foundProvider},
		}},
		categoryFinderWithExistingCategory(),
		&profilePhotoValidatorMock{},
		profileReader,
		nil,
		&identityApprovalReaderMock{},
	)

	results, err := providerService.SearchProvidersByCategoryID(context.Background(), providerCategory.ID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 0.0, results[0].RatingAverage)
	assert.Equal(t, 0, results[0].RatingCount)
}

func TestServicePropagatesProviderSearchRatingReaderError(t *testing.T) {
	providerCategory := existingCategory()
	foundProvider, err := provider.NewProvider(
		"auth0|juan",
		"juan@example.com",
		"Juan",
		"Pérez",
		&providerCategory,
		&filedomain.Image{FileID: "juan-photo"},
		[]coveragezone.CoverageZone{defaultCoverageZone()},
	)
	require.NoError(t, err)
	foundProvider.SetPersistenceID(12)
	expectedErr := errors.New("rating reader unavailable")
	providerService := provider.NewService(
		&providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*foundProvider},
		}},
		categoryFinderWithExistingCategory(),
		&profilePhotoValidatorMock{},
		&providerProfileReaderMock{batchStatsErr: expectedErr},
		nil,
		&identityApprovalReaderMock{},
	)

	results, err := providerService.SearchProvidersByCategoryID(context.Background(), providerCategory.ID)

	assert.Nil(t, results)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "finding provider rating stats for search")
}

func TestServicePropagatesProviderSearchIdentityApprovalReaderError(t *testing.T) {
	providerCategory := existingCategory()
	foundProvider, err := provider.NewProvider(
		"auth0|juan",
		"juan@example.com",
		"Juan",
		"Pérez",
		&providerCategory,
		&filedomain.Image{FileID: "juan-photo"},
		[]coveragezone.CoverageZone{defaultCoverageZone()},
	)
	require.NoError(t, err)
	foundProvider.SetPersistenceID(12)
	expectedErr := errors.New("identity approval reader unavailable")
	providerService := provider.NewService(
		&providerRepositoryMock{providersByCategoryID: map[int][]provider.Provider{
			providerCategory.ID: {*foundProvider},
		}},
		categoryFinderWithExistingCategory(),
		&profilePhotoValidatorMock{},
		&providerProfileReaderMock{statsByProviderID: map[int]provider.RatingStats{}},
		nil,
		&identityApprovalReaderMock{err: expectedErr},
	)

	results, err := providerService.SearchProvidersByCategoryID(context.Background(), providerCategory.ID)

	assert.Nil(t, results)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "finding approved provider identities for search")
}
