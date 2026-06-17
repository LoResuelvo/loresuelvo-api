package file_test

import (
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validFileMetadata(t *testing.T) filedomain.FileMetadata {
	t.Helper()

	metadata, err := filedomain.NewFileMetadata("foto.jpg", "image/jpeg", 1024)
	require.NoError(t, err)
	return *metadata
}

func validFileTime() time.Time {
	return time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
}

func TestNewFileMetadataStoresValues(t *testing.T) {
	metadata, err := filedomain.NewFileMetadata(" foto.jpg ", "image/jpeg", 1024)

	require.NoError(t, err)
	assert.Equal(t, " foto.jpg ", metadata.OriginalName())
	assert.Equal(t, "image/jpeg", metadata.MimeType())
	assert.Equal(t, 1024, metadata.SizeBytes())
}

func TestNewFileMetadataValidatesRequiredFields(t *testing.T) {
	tests := []struct {
		name         string
		originalName string
		mimeType     string
		sizeBytes    int
		expectedErr  error
	}{
		{name: "original name", mimeType: "image/jpeg", sizeBytes: 1024, expectedErr: filedomain.ErrOriginalNameRequired},
		{name: "mime type", originalName: "foto.jpg", sizeBytes: 1024, expectedErr: filedomain.ErrMimeTypeRequired},
		{name: "size", originalName: "foto.jpg", mimeType: "image/jpeg", expectedErr: filedomain.ErrSizeRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := filedomain.NewFileMetadata(tt.originalName, tt.mimeType, tt.sizeBytes)

			assert.Nil(t, metadata)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestNewPendingFileCreatesPendingFile(t *testing.T) {
	now := validFileTime()
	metadata := validFileMetadata(t)

	file, err := filedomain.NewPendingFile(
		"file-id",
		"files/2026/06/provider_profile_photo/file-id.jpg",
		"public",
		metadata,
		filedomain.VisibilityPublic,
		filedomain.PurposeProviderProfilePhoto,
		"auth0|provider",
		now,
	)

	require.NoError(t, err)
	assert.Equal(t, "file-id", file.ID)
	assert.Equal(t, filedomain.StatusPending, file.Status)
	assert.Equal(t, now, file.CreatedOn)
	assert.Equal(t, now, file.UpdatedOn)
}

func TestNewPendingFileReturnsValidationError(t *testing.T) {
	file, err := filedomain.NewPendingFile("", "key", "bucket", validFileMetadata(t), filedomain.VisibilityPublic, filedomain.PurposeProviderProfilePhoto, "auth0|provider", validFileTime())

	assert.Nil(t, file)
	assert.ErrorIs(t, err, filedomain.ErrFileIDRequired)
}

func TestNewFileCreatesFileAndExposesState(t *testing.T) {
	createdOn := validFileTime()
	updatedOn := createdOn.Add(time.Hour)
	metadata := validFileMetadata(t)

	file, err := filedomain.NewFile(
		"file-id",
		"files/2026/06/provider_profile_photo/file-id.jpg",
		"public",
		metadata,
		filedomain.StatusConfirmed,
		filedomain.VisibilityPublic,
		filedomain.PurposeProviderProfilePhoto,
		"auth0|provider",
		createdOn,
		updatedOn,
	)

	require.NoError(t, err)
	assert.Equal(t, "foto.jpg", file.OriginalName())
	assert.Equal(t, "image/jpeg", file.MimeType())
	assert.Equal(t, 1024, file.SizeBytes())
	assert.Equal(t, metadata, file.Metadata())
	assert.True(t, file.IsConfirmed())
	assert.True(t, file.IsPublic())
	assert.True(t, file.WasUploadedBy("auth0|provider"))
	assert.True(t, file.HasPurpose(filedomain.PurposeProviderProfilePhoto))
}

func TestNewFileValidatesRequiredFields(t *testing.T) {
	now := validFileTime()
	metadata := validFileMetadata(t)
	tests := []struct {
		name        string
		id          string
		key         string
		bucket      string
		status      string
		visibility  string
		purpose     string
		uploader    string
		createdOn   time.Time
		updatedOn   time.Time
		expectedErr error
	}{
		{name: "id", key: "key", bucket: "bucket", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", createdOn: now, updatedOn: now, expectedErr: filedomain.ErrFileIDRequired},
		{name: "status", id: "file-id", key: "key", bucket: "bucket", visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", createdOn: now, updatedOn: now, expectedErr: filedomain.ErrFileStatusRequired},
		{name: "key", id: "file-id", bucket: "bucket", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", createdOn: now, updatedOn: now, expectedErr: filedomain.ErrFileKeyRequired},
		{name: "bucket", id: "file-id", key: "key", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", createdOn: now, updatedOn: now, expectedErr: filedomain.ErrFileBucketRequired},
		{name: "visibility", id: "file-id", key: "key", bucket: "bucket", status: filedomain.StatusPending, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", createdOn: now, updatedOn: now, expectedErr: filedomain.ErrVisibilityRequired},
		{name: "purpose", id: "file-id", key: "key", bucket: "bucket", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, uploader: "auth0|provider", createdOn: now, updatedOn: now, expectedErr: filedomain.ErrPurposeRequired},
		{name: "uploader", id: "file-id", key: "key", bucket: "bucket", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, createdOn: now, updatedOn: now, expectedErr: filedomain.ErrUploaderRequired},
		{name: "created timestamp", id: "file-id", key: "key", bucket: "bucket", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", updatedOn: now, expectedErr: filedomain.ErrFileTimestampRequired},
		{name: "updated timestamp", id: "file-id", key: "key", bucket: "bucket", status: filedomain.StatusPending, visibility: filedomain.VisibilityPublic, purpose: filedomain.PurposeProviderProfilePhoto, uploader: "auth0|provider", createdOn: now, expectedErr: filedomain.ErrFileTimestampRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := filedomain.NewFile(tt.id, tt.key, tt.bucket, metadata, tt.status, tt.visibility, tt.purpose, tt.uploader, tt.createdOn, tt.updatedOn)

			assert.Nil(t, file)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestConfirmMarksFileConfirmed(t *testing.T) {
	now := validFileTime()
	file, err := filedomain.NewPendingFile("file-id", "key", "bucket", validFileMetadata(t), filedomain.VisibilityPublic, filedomain.PurposeProviderProfilePhoto, "auth0|provider", now)
	require.NoError(t, err)
	confirmedOn := now.Add(time.Hour)

	file.Confirm(confirmedOn)

	assert.Equal(t, filedomain.StatusConfirmed, file.Status)
	assert.Equal(t, confirmedOn, file.UpdatedOn)
}

func TestUploadPolicyAllowsSupportedMetadataWithinSize(t *testing.T) {
	metadata := validFileMetadata(t)
	policy := filedomain.UploadPolicy{
		MaxSizeBytes:     1024,
		AllowedMimeTypes: map[string]struct{}{"image/jpeg": {}},
	}

	assert.True(t, policy.Allows(metadata))
}

func TestUploadPolicyRejectsUnsupportedMetadata(t *testing.T) {
	metadata := validFileMetadata(t)
	policy := filedomain.UploadPolicy{
		MaxSizeBytes:     1023,
		AllowedMimeTypes: map[string]struct{}{"image/png": {}},
	}

	assert.False(t, policy.Allows(metadata))
}

func TestSystemClockReturnsUTCTime(t *testing.T) {
	now := clockadapter.SystemClock{}.Now()

	assert.Equal(t, time.UTC, now.Location())
	assert.False(t, now.IsZero())
}
