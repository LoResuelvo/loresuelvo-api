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

func TestServiceIncludesProviderRatingSummary(t *testing.T) {
	providerService, _ := newProviderServiceWithProfileReader(t, provider.RatingStats{Total: 9, Count: 2}, nil)

	profile, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	require.NoError(t, err)
	assert.Equal(t, 4.5, profile.RatingAverage)
	assert.Equal(t, 2, profile.RatingCount)
}

func TestServiceReturnsEmptyWorkHistoryWhenProviderHasNoPaidOrders(t *testing.T) {
	providerService, _ := newProviderServiceWithProfileReader(t, provider.RatingStats{}, nil)

	profile, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	require.NoError(t, err)
	assert.Equal(t, []readmodel.WorkOrder{}, profile.WorkOrders)
}

func TestServiceIncludesPaidWorkOrderWithoutReview(t *testing.T) {
	workOrder := readmodel.WorkOrder{ID: 41, Status: "paid"}
	providerService, _ := newProviderServiceWithProfileReader(t, provider.RatingStats{}, []readmodel.WorkOrder{workOrder})

	profile, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	require.NoError(t, err)
	assert.Equal(t, []readmodel.WorkOrder{workOrder}, profile.WorkOrders)
}

func TestServiceIncludesReviewInPaidWorkOrder(t *testing.T) {
	review := &readmodel.Review{Rating: 5, Description: "Excelente trabajo"}
	providerService, _ := newProviderServiceWithProfileReader(t, provider.RatingStats{}, []readmodel.WorkOrder{
		{ID: 41, Status: "paid", Review: review},
	})

	profile, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	require.NoError(t, err)
	require.Len(t, profile.WorkOrders, 1)
	assert.Equal(t, review, profile.WorkOrders[0].Review)
}

func TestServiceIncludesIdentityApprovalInPublicProfile(t *testing.T) {
	identityReader := &identityApprovalReaderMock{
		approvedByProviderID: map[int]bool{12: true},
	}
	providerService := provider.NewService(
		&providerRepositoryMock{providerByID: providerForProfileService(t)},
		categoryFinderWithExistingCategory(),
		profilePhotoServiceForProfile(t),
		&providerProfileReaderMock{},
		nil,
		identityReader,
	)

	profile, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	require.NoError(t, err)
	assert.True(t, profile.IdentityVerified)
	assert.Equal(t, []int{12}, identityReader.providerIDs)
}

func TestServicePropagatesRatingStatsError(t *testing.T) {
	expectedErr := errors.New("rating stats unavailable")
	repository := &providerRepositoryMock{providerByID: providerForProfileService(t)}
	providerService := provider.NewService(
		repository,
		categoryFinderWithExistingCategory(),
		profilePhotoServiceForProfile(t),
		&providerProfileReaderMock{ratingStatsErr: expectedErr},
		nil,
	)

	_, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	assert.ErrorIs(t, err, expectedErr)
}

func TestServicePropagatesPaidWorkHistoryError(t *testing.T) {
	expectedErr := errors.New("work history unavailable")
	repository := &providerRepositoryMock{providerByID: providerForProfileService(t)}
	providerService := provider.NewService(
		repository,
		categoryFinderWithExistingCategory(),
		profilePhotoServiceForProfile(t),
		&providerProfileReaderMock{workHistoryErr: expectedErr},
		nil,
	)

	_, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	assert.ErrorIs(t, err, expectedErr)
}

func TestServicePropagatesIdentityApprovalErrorInPublicProfile(t *testing.T) {
	expectedErr := errors.New("identity approval unavailable")
	providerService := provider.NewService(
		&providerRepositoryMock{providerByID: providerForProfileService(t)},
		categoryFinderWithExistingCategory(),
		profilePhotoServiceForProfile(t),
		&providerProfileReaderMock{},
		nil,
		&identityApprovalReaderMock{err: expectedErr},
	)

	_, err := providerService.GetProviderProfileDetail(context.Background(), 12)

	assert.ErrorIs(t, err, expectedErr)
}

func newProviderServiceWithProfileReader(t *testing.T, stats provider.RatingStats, workOrders []readmodel.WorkOrder) (*provider.Service, *providerRepositoryMock) {
	t.Helper()
	repository := &providerRepositoryMock{providerByID: providerForProfileService(t)}
	providerService := provider.NewService(
		repository,
		categoryFinderWithExistingCategory(),
		profilePhotoServiceForProfile(t),
		&providerProfileReaderMock{stats: stats, workOrders: workOrders},
		nil,
		&identityApprovalReaderMock{},
	)

	return providerService, repository
}

func providerForProfileService(t *testing.T) *provider.Provider {
	t.Helper()
	providerCategory := existingCategory()
	foundProvider, err := provider.NewProvider(
		"auth0|juan",
		"juan@example.com",
		"Juan",
		"Gómez",
		&providerCategory,
		&filedomain.Image{FileID: "profile-photo-id"},
		[]coveragezone.CoverageZone{defaultCoverageZone()},
	)
	require.NoError(t, err)
	foundProvider.SetPersistenceID(12)

	return foundProvider
}

func profilePhotoServiceForProfile(t *testing.T) *profilePhotoValidatorMock {
	t.Helper()
	return &profilePhotoValidatorMock{profilePhotoURLsByFile: map[string]string{
		"profile-photo-id": "https://cdn.example/juan.jpg",
	}}
}
