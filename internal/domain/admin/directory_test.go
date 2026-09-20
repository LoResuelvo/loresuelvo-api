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

func TestDirectoryServiceListsConsumersWithProfilePhotoURLsResolvedInBulk(t *testing.T) {
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
	service := admin.NewDirectoryService(reader, resolver)

	result, err := service.ListConsumers(t.Context())

	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, "https://cdn.example/photo-1.jpg", result[0].ProfilePhotoURL)
	assert.Empty(t, result[1].ProfilePhotoURL)
	assert.Equal(t, "https://cdn.example/photo-3.jpg", result[2].ProfilePhotoURL)
	reader.AssertExpectations(t)
	resolver.AssertExpectations(t)
}

func TestDirectoryServiceReturnsNonNilEmptyConsumerDirectoryWithoutResolvingPhotos(t *testing.T) {
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return([]readmodel.Consumer{}, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	service := admin.NewDirectoryService(reader, resolver)

	result, err := service.ListConsumers(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
	resolver.AssertNotCalled(t, "ResolvePublicURLs", mock.Anything, mock.Anything)
	reader.AssertExpectations(t)
}

func TestDirectoryServiceDoesNotResolvePhotosWhenConsumersHaveNone(t *testing.T) {
	consumers := []readmodel.Consumer{{ID: 1, Email: "ana@example.com"}}
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(consumers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	service := admin.NewDirectoryService(reader, resolver)

	result, err := service.ListConsumers(t.Context())

	require.NoError(t, err)
	assert.Equal(t, consumers, result)
	resolver.AssertNotCalled(t, "ResolvePublicURLs", mock.Anything, mock.Anything)
	reader.AssertExpectations(t)
}

func TestDirectoryServiceWrapsConsumerReaderError(t *testing.T) {
	expectedErr := errors.New("database unavailable")
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(nil, expectedErr).Once()
	service := admin.NewDirectoryService(reader, new(profilePhotoURLResolverMock))

	result, err := service.ListConsumers(t.Context())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "finding consumers for administrative directory")
	reader.AssertExpectations(t)
}

func TestDirectoryServiceWrapsProfilePhotoResolutionError(t *testing.T) {
	expectedErr := errors.New("storage unavailable")
	consumers := []readmodel.Consumer{{ID: 1, Email: "ana@example.com", ProfilePhotoFileID: "photo-1"}}
	reader := new(consumerDirectoryReaderMock)
	reader.On("FindConsumers", mock.Anything).Return(consumers, nil).Once()
	resolver := new(profilePhotoURLResolverMock)
	resolver.On("ResolvePublicURLs", mock.Anything, []string{"photo-1"}).Return(nil, expectedErr).Once()
	service := admin.NewDirectoryService(reader, resolver)

	result, err := service.ListConsumers(t.Context())

	assert.Nil(t, result)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "resolving consumer profile photo URLs")
	reader.AssertExpectations(t)
	resolver.AssertExpectations(t)
}
