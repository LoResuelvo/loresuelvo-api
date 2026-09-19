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
	"github.com/stretchr/testify/require"
)

func TestProviderSearchResolvesPhotosAndPreservesReadModel(t *testing.T) {
	reader := &providerSearchReaderMock{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	expected := []readmodel.ProviderSearchResult{{ID: 12, Name: "Ana", Surname: "Perez", CategoryName: "Plumbing",
		CoverageZones: []coveragezone.CoverageZone{defaultCoverageZone()},
		ProfilePhoto:  &filedomain.Image{FileID: "photo", OriginalName: "photo.jpg"}, RatingAverage: 4.7, RatingCount: 3, IdentityVerified: true}, {ID: 13}}
	reader.On("FindByCategoryID", ctx, 1).Return(expected, nil).Once()
	files := &profilePhotoValidatorMock{profilePhotoURLsByFile: map[string]string{"photo": "https://cdn.example/photo.jpg"}}
	service := provider.NewService(reader, nil, categoryFinderWithExistingCategory(), files, nil, nil)
	results, err := service.SearchProvidersByCategoryID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, expected, results)
	require.Equal(t, "https://cdn.example/photo.jpg", results[0].ProfilePhoto.URL)
	require.Equal(t, []string{"photo"}, files.resolvedFileIDs)
	reader.AssertExpectations(t)
}

func TestProviderSearchValidatesCategoryBeforeReading(t *testing.T) {
	for _, id := range []int{-1, 0, 2} {
		service := provider.NewService(nil, nil, categoryFinderWithExistingCategory(), nil, nil, nil)
		results, err := service.SearchProvidersByCategoryID(t.Context(), id)
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
		reader.On("FindByCategoryID", t.Context(), 1).Return(results, nil).Once()
		service := provider.NewService(reader, nil, categoryFinderWithExistingCategory(), nil, nil, nil)
		actual, err := service.SearchProvidersByCategoryID(t.Context(), 1)
		require.NoError(t, err)
		require.Equal(t, results, actual)
		reader.AssertExpectations(t)
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
			reader.On("FindByCategoryID", t.Context(), 1).Return([]readmodel.ProviderSearchResult{{ID: 1, ProfilePhoto: &filedomain.Image{FileID: "photo"}}}, readErr).Once()
			service := provider.NewService(reader, nil, categoryFinderWithExistingCategory(), &profilePhotoValidatorMock{resolveErr: failure}, nil, nil)
			results, err := service.SearchProvidersByCategoryID(t.Context(), 1)
			require.ErrorIs(t, err, failure)
			require.Nil(t, results)
			reader.AssertExpectations(t)
		})
	}
}

func TestProviderSearchRequiresReader(t *testing.T) {
	service := provider.NewService(nil, nil, categoryFinderWithExistingCategory(), nil, nil, nil)
	results, err := service.SearchProvidersByCategoryID(t.Context(), 1)
	require.Nil(t, results)
	require.ErrorIs(t, err, provider.ErrSearchReaderNotConfigured)
}
