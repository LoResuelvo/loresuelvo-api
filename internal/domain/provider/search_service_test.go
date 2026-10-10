package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProviderSearchResolvesPhotosAndPreservesReadModel(t *testing.T) {
	reader := &providerSearchReaderMock{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	expected := []readmodel.ProviderSearchResult{{ID: 12, Name: "Ana", Surname: "Perez", CategoryName: "Plumbing",
		CoverageZones: []coveragezone.CoverageZone{defaultCoverageZone()},
		ProfilePhoto:  &filedomain.Image{FileID: "photo", OriginalName: "photo.jpg"}, RatingAverage: 4.7, RatingCount: 3, IdentityVerified: true}, {ID: 13}}
	reader.On("FindByCategoryAndCoverageZoneID", ctx, 1, 14).Return(expected, nil).Once()
	files := &profilePhotoValidatorMock{profilePhotoURLsByFile: map[string]string{"photo": "https://cdn.example/photo.jpg"}}
	consumers := searchConsumerFinder(ctx, "auth0|consumer", 14)
	service := provider.NewService(nil, reader, nil, categoryFinderWithExistingCategory(), files, nil, nil, consumers)
	results, err := service.SearchProvidersByCategoryID(ctx, "auth0|consumer", 1)
	require.NoError(t, err)
	require.Equal(t, expected, results)
	require.Equal(t, "https://cdn.example/photo.jpg", results[0].ProfilePhoto.URL)
	require.Equal(t, []string{"photo"}, files.resolvedFileIDs)
	reader.AssertExpectations(t)
	consumers.AssertExpectations(t)
}

func TestProviderSearchValidatesCategoryBeforeReading(t *testing.T) {
	for _, id := range []int{-1, 0, 2} {
		service := newProviderServiceForTest(nil, nil, categoryFinderWithExistingCategory(), nil, nil, nil)
		results, err := service.SearchProvidersByCategoryID(t.Context(), "auth0|consumer", id)
		require.Nil(t, results)
		if id <= 0 {
			require.ErrorIs(t, err, category.ErrIDRequired)
		} else {
			require.ErrorIs(t, err, category.ErrDoesNotExist)
		}
	}
}

func TestProviderSearchSkipsFilesWithoutPhotos(t *testing.T) {
	for _, results := range [][]readmodel.ProviderSearchResult{{}, {{ID: 1, ProfilePhoto: &filedomain.Image{}}}, {{ID: 1}}} {
		reader := &providerSearchReaderMock{}
		reader.On("FindByCategoryAndCoverageZoneID", t.Context(), 1, 14).Return(results, nil).Once()
		consumers := searchConsumerFinder(t.Context(), "auth0|consumer", 14)
		service := provider.NewService(nil, reader, nil, categoryFinderWithExistingCategory(), nil, nil, nil, consumers)
		actual, err := service.SearchProvidersByCategoryID(t.Context(), "auth0|consumer", 1)
		require.NoError(t, err)
		require.Equal(t, results, actual)
		reader.AssertExpectations(t)
		consumers.AssertExpectations(t)
	}
}

func TestProviderSearchPropagatesErrors(t *testing.T) {
	failure := errors.New("unavailable")
	for _, stage := range []string{"reader", "files"} {
		t.Run(stage, func(t *testing.T) {
			reader := &providerSearchReaderMock{}
			var readErr error
			if stage == "reader" {
				readErr = failure
			}
			reader.On("FindByCategoryAndCoverageZoneID", t.Context(), 1, 14).Return([]readmodel.ProviderSearchResult{{ID: 1, ProfilePhoto: &filedomain.Image{FileID: "photo"}}}, readErr).Once()
			consumers := searchConsumerFinder(t.Context(), "auth0|consumer", 14)
			service := provider.NewService(nil, reader, nil, categoryFinderWithExistingCategory(), &profilePhotoValidatorMock{resolveErr: failure}, nil, nil, consumers)
			results, err := service.SearchProvidersByCategoryID(t.Context(), "auth0|consumer", 1)
			require.ErrorIs(t, err, failure)
			require.Nil(t, results)
			reader.AssertExpectations(t)
			consumers.AssertExpectations(t)
		})
	}
}

func TestProviderSearchRequiresReader(t *testing.T) {
	service := newProviderServiceForTest(nil, nil, categoryFinderWithExistingCategory(), nil, nil, nil)
	results, err := service.SearchProvidersByCategoryID(t.Context(), "auth0|consumer", 1)
	require.Nil(t, results)
	require.ErrorIs(t, err, provider.ErrSearchReaderNotConfigured)
}

func TestProviderSearchPropagatesConsumerLookupFailure(t *testing.T) {
	failure := errors.New("consumer unavailable")
	consumers := new(consumerFinderMock)
	consumers.On("FindConsumerByAuthID", t.Context(), "auth0|consumer").Return(nil, failure).Once()
	reader := new(providerSearchReaderMock)
	service := provider.NewService(nil, reader, nil, categoryFinderWithExistingCategory(), nil, nil, nil, consumers)
	results, err := service.SearchProvidersByCategoryID(t.Context(), "auth0|consumer", 1)
	require.ErrorIs(t, err, failure)
	require.Nil(t, results)
	consumers.AssertExpectations(t)
	reader.AssertNotCalled(t, "FindByCategoryAndCoverageZoneID", mock.Anything, mock.Anything, mock.Anything)
}
