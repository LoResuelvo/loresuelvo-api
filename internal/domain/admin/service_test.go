package admin_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServiceListsConsumersWithProfilePhotoURLsResolvedInBulk(t *testing.T) {
	createdOn := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	consumers := []readmodel.Consumer{
		{ID: 1, Email: "ana@example.com", ProfilePhotoFileID: "photo-1", CreatedOn: createdOn},
		{ID: 2, Email: "beatriz@example.com", CreatedOn: createdOn.Add(time.Minute)},
		{ID: 3, Email: "carla@example.com", ProfilePhotoFileID: "photo-3", CreatedOn: createdOn.Add(2 * time.Minute)},
	}
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(consumers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	resolver.On("ResolvePublicURLs", mock.Anything, []string{"photo-1", "photo-3"}).Return(map[string]string{
		"photo-1": "https://cdn.example/photo-1.jpg",
		"photo-3": "https://cdn.example/photo-3.jpg",
	}, nil).Once()
	service := admin.NewService(reader, nil, resolver)

	result, err := service.ListConsumers(t.Context())

	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, "https://cdn.example/photo-1.jpg", result[0].ProfilePhotoURL)
	assert.Empty(t, result[1].ProfilePhotoURL)
	assert.Equal(t, "https://cdn.example/photo-3.jpg", result[2].ProfilePhotoURL)
	reader.AssertExpectations(t)
	resolver.AssertExpectations(t)
}

func TestServiceReturnsNonNilEmptyConsumerDirectoryWithoutResolvingPhotos(t *testing.T) {
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return([]readmodel.Consumer{}, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	service := admin.NewService(reader, nil, resolver)

	result, err := service.ListConsumers(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
	resolver.AssertNotCalled(t, "ResolvePublicURLs", mock.Anything, mock.Anything)
	reader.AssertExpectations(t)
}

func TestServiceDoesNotResolvePhotosWhenConsumersHaveNone(t *testing.T) {
	consumers := []readmodel.Consumer{{ID: 1, Email: "ana@example.com"}}
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(consumers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	service := admin.NewService(reader, nil, resolver)

	result, err := service.ListConsumers(t.Context())

	require.NoError(t, err)
	assert.Equal(t, consumers, result)
	resolver.AssertNotCalled(t, "ResolvePublicURLs", mock.Anything, mock.Anything)
	reader.AssertExpectations(t)
}

func TestServiceWrapsConsumerReaderError(t *testing.T) {
	expectedErr := errors.New("database unavailable")
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(nil, expectedErr).Once()
	service := admin.NewService(reader, nil, new(profilePhotoURLResolverMock))

	result, err := service.ListConsumers(t.Context())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "finding consumers for administrative directory")
	reader.AssertExpectations(t)
}

func TestServiceWrapsProfilePhotoResolutionError(t *testing.T) {
	expectedErr := errors.New("storage unavailable")
	consumers := []readmodel.Consumer{{ID: 1, Email: "ana@example.com", ProfilePhotoFileID: "photo-1"}}
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(consumers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	resolver.On("ResolvePublicURLs", mock.Anything, []string{"photo-1"}).Return(nil, expectedErr).Once()
	service := admin.NewService(reader, nil, resolver)

	result, err := service.ListConsumers(t.Context())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "resolving consumer profile photo URLs")
	reader.AssertExpectations(t)
	resolver.AssertExpectations(t)
}

func TestServiceListsProvidersWithProfilePhotoURLsResolvedInBulk(t *testing.T) {
	providers := []readmodel.Provider{
		{ID: 1, Email: "juan@example.com", ProfilePhotoFileID: "photo-1"},
		{ID: 2, Email: "laura@example.com"},
		{ID: 3, Email: "pedro@example.com", ProfilePhotoFileID: "photo-3"},
	}
	reader := new(providerDirectoryReaderMock)
	reader.On("FindProviders", mock.Anything).Return(providers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	resolver.On("ResolvePublicURLs", mock.Anything, []string{"photo-1", "photo-3"}).Return(map[string]string{
		"photo-1": "https://cdn.example/photo-1.jpg",
		"photo-3": "https://cdn.example/photo-3.jpg",
	}, nil).Once()
	service := admin.NewService(nil, reader, resolver)

	result, err := service.ListProviders(t.Context())

	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, "https://cdn.example/photo-1.jpg", result[0].ProfilePhotoURL)
	assert.Empty(t, result[1].ProfilePhotoURL)
	assert.Equal(t, "https://cdn.example/photo-3.jpg", result[2].ProfilePhotoURL)
	reader.AssertExpectations(t)
	resolver.AssertExpectations(t)
}

func TestServiceReturnsNonNilEmptyProviderDirectoryWithoutResolvingPhotos(t *testing.T) {
	reader := new(providerDirectoryReaderMock)
	reader.On("FindProviders", mock.Anything).Return([]readmodel.Provider{}, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	service := admin.NewService(nil, reader, resolver)

	result, err := service.ListProviders(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
	resolver.AssertNotCalled(t, "ResolvePublicURLs", mock.Anything, mock.Anything)
	reader.AssertExpectations(t)
}

func TestServiceDoesNotResolvePhotosWhenProvidersHaveNone(t *testing.T) {
	providers := []readmodel.Provider{{ID: 1, Email: "juan@example.com"}}
	reader := new(providerDirectoryReaderMock)
	reader.On("FindProviders", mock.Anything).Return(providers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	service := admin.NewService(nil, reader, resolver)

	result, err := service.ListProviders(t.Context())

	require.NoError(t, err)
	assert.Equal(t, providers, result)
	resolver.AssertNotCalled(t, "ResolvePublicURLs", mock.Anything, mock.Anything)
	reader.AssertExpectations(t)
}

func TestServiceWrapsProviderReaderError(t *testing.T) {
	expectedErr := errors.New("database unavailable")
	reader := new(providerDirectoryReaderMock)
	reader.On("FindProviders", mock.Anything).Return(nil, expectedErr).Once()
	service := admin.NewService(nil, reader, new(profilePhotoURLResolverMock))

	result, err := service.ListProviders(t.Context())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "finding providers for administrative directory")
	reader.AssertExpectations(t)
}

func TestServiceWrapsProviderProfilePhotoResolutionError(t *testing.T) {
	expectedErr := errors.New("storage unavailable")
	providers := []readmodel.Provider{{ID: 1, Email: "juan@example.com", ProfilePhotoFileID: "photo-1"}}
	reader := new(providerDirectoryReaderMock)
	reader.On("FindProviders", mock.Anything).Return(providers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	resolver.On("ResolvePublicURLs", mock.Anything, []string{"photo-1"}).Return(nil, expectedErr).Once()
	service := admin.NewService(nil, reader, resolver)

	result, err := service.ListProviders(t.Context())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "resolving provider profile photo URLs")
	reader.AssertExpectations(t)
	resolver.AssertExpectations(t)
}
