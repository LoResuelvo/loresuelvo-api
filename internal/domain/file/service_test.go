package file_test

import (
	"context"
	"errors"
	"testing"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fileRepositoryMock struct {
	files        map[string]filedomain.File
	saveErr      error
	findByIDErr  error
	findByIDsErr error
}

func newFileRepositoryMock() *fileRepositoryMock {
	return &fileRepositoryMock{files: map[string]filedomain.File{}}
}

func (repo *fileRepositoryMock) Save(_ context.Context, file filedomain.File) error {
	if repo.saveErr != nil {
		return repo.saveErr
	}
	repo.files[file.ID] = file
	return nil
}

func (repo *fileRepositoryMock) FindByID(_ context.Context, id string) (*filedomain.File, error) {
	if repo.findByIDErr != nil {
		return nil, repo.findByIDErr
	}
	file, ok := repo.files[id]
	if !ok {
		return nil, assert.AnError
	}
	return &file, nil
}

func (repo *fileRepositoryMock) FindByIDs(_ context.Context, ids []string) ([]filedomain.File, error) {
	if repo.findByIDsErr != nil {
		return nil, repo.findByIDsErr
	}
	files := make([]filedomain.File, 0, len(ids))
	for _, id := range ids {
		file, ok := repo.files[id]
		if ok {
			files = append(files, file)
		}
	}
	return files, nil
}

func (repo *fileRepositoryMock) DeleteAll() error { return nil }

type storageMock struct {
	metadataByObject map[string]filedomain.ObjectMetadata
	generateErr      error
	readErr          error
}

func newStorageMock() *storageMock {
	return &storageMock{metadataByObject: map[string]filedomain.ObjectMetadata{}}
}

func (storage *storageMock) GenerateUploadURL(_ context.Context, object filedomain.ObjectToUpload) (*filedomain.UploadTarget, error) {
	if storage.generateErr != nil {
		return nil, storage.generateErr
	}
	storage.metadataByObject[object.Bucket+"/"+object.Key] = filedomain.ObjectMetadata{MimeType: object.MimeType, SizeBytes: object.SizeBytes}
	return &filedomain.UploadTarget{URL: "https://upload", Headers: map[string]string{"Content-Type": object.MimeType}}, nil
}

func (storage *storageMock) ReadObjectMetadata(_ context.Context, bucket, key string) (*filedomain.ObjectMetadata, error) {
	if storage.readErr != nil {
		return nil, storage.readErr
	}
	metadata, ok := storage.metadataByObject[bucket+"/"+key]
	if !ok {
		return nil, assert.AnError
	}
	return &metadata, nil
}

func (storage *storageMock) PublicURL(bucket, key string) string {
	return "https://cdn/" + bucket + "/" + key
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
}

func newFileService(repo *fileRepositoryMock, storage *storageMock) *filedomain.Service {
	return filedomain.NewService(repo, storage, "public", "private", fixedClock{})
}

func TestNewServiceRequiresClock(t *testing.T) {
	assert.PanicsWithValue(t, "file clock is required", func() {
		filedomain.NewService(newFileRepositoryMock(), newStorageMock(), "public", "private", nil)
	})
}

func TestRequestUploadCreatesPendingProviderProfilePhoto(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())

	result, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.FileID)
	assert.Contains(t, result.Key, "files/2026/06/provider_profile_photo/")
	assert.Equal(t, "https://upload", result.URL)
	createdFile := repo.files[result.FileID]
	assert.Equal(t, filedomain.StatusPending, createdFile.Status)
	assert.Equal(t, filedomain.VisibilityPublic, createdFile.Visibility)
	assert.Equal(t, "auth0|provider", createdFile.UploadedByAuthID)
}

func TestRequestUploadRejectsMissingPurpose(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
	})

	assert.ErrorIs(t, err, filedomain.ErrPurposeRequired)
}

func TestRequestUploadRejectsUnsupportedPurpose(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      "unknown",
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedPurpose)
}

func TestRequestUploadRequiresUploader(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	assert.ErrorIs(t, err, filedomain.ErrUploaderRequired)
}

func TestRequestUploadWrapsStorageError(t *testing.T) {
	expectedErr := errors.New("storage unavailable")
	service := newFileService(newFileRepositoryMock(), &storageMock{generateErr: expectedErr})

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "generating upload url")
}

func TestRequestUploadWrapsRepositorySaveError(t *testing.T) {
	expectedErr := errors.New("repository unavailable")
	repo := newFileRepositoryMock()
	repo.saveErr = expectedErr
	service := newFileService(repo, newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "saving pending file")
}

func TestRequestUploadAcceptsWebPProviderProfilePhoto(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.webp",
		MimeType:     "image/webp",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	assert.NoError(t, err)
}

func TestRequestUploadRejectsInvalidProfilePhotoMimeType(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.gif",
		MimeType:     "image/gif",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedProfilePhoto)
}

func TestRequestUploadRequiresOriginalName(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:    "auth0|provider",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
		Purpose:   filedomain.PurposeProviderProfilePhoto,
	})

	assert.ErrorIs(t, err, filedomain.ErrOriginalNameRequired)
}

func TestRequestUploadRejectsOversizedProfilePhoto(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    6 * 1024 * 1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedProfilePhoto)
}

func TestConfirmUploadMarksFileAsConfirmed(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)

	confirmed, err := service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/png",
		SizeBytes: 2048,
	})

	require.NoError(t, err)
	assert.Equal(t, upload.FileID, confirmed.FileID)
	assert.Equal(t, "https://cdn/public/"+upload.Key, confirmed.URL)
	assert.NoError(t, service.ValidateProviderProfilePhoto(context.Background(), "auth0|provider", upload.FileID))
}

func TestConfirmUploadRejectsUnavailableFile(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())

	_, err := service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    "missing",
		Key:       "key",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
	})

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestConfirmUploadRejectsMismatchedRequestData(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|other",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/png",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestConfirmUploadRejectsMismatchedMetadataRequestData(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/jpeg",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestConfirmUploadRejectsUnreadableObjectMetadata(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)
	storage.readErr = errors.New("metadata unavailable")

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/png",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestConfirmUploadRejectsMismatchedObjectMetadata(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)
	storage.metadataByObject["public/"+upload.Key] = filedomain.ObjectMetadata{MimeType: "image/jpeg", SizeBytes: 2048}

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/png",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestConfirmUploadWrapsRepositorySaveError(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)
	expectedErr := errors.New("repository unavailable")
	repo.saveErr = expectedErr

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/png",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "confirming file upload")
}

func TestValidateProviderProfilePhotoRequiresConfirmedOwnerFile(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)

	assert.ErrorIs(t, service.ValidateProviderProfilePhoto(context.Background(), "auth0|provider", ""), filedomain.ErrProfilePhotoRequired)
	assert.ErrorIs(t, service.ValidateProviderProfilePhoto(context.Background(), "auth0|provider", upload.FileID), filedomain.ErrProfilePhotoNotAvailable)
}

func TestValidateProviderProfilePhotoRejectsWrongOwner(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)
	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
	})
	require.NoError(t, err)

	err = service.ValidateProviderProfilePhoto(context.Background(), "auth0|other", upload.FileID)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestValidateProviderProfilePhotoRejectsMissingFile(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	err := service.ValidateProviderProfilePhoto(context.Background(), "auth0|provider", "missing")

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestResolvePublicURLsReturnsConfirmedPublicFileURLsInBatch(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)
	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
	})
	require.NoError(t, err)

	urls, err := service.ResolvePublicURLs(context.Background(), []string{upload.FileID, upload.FileID, ""})

	require.NoError(t, err)
	assert.Equal(t, "https://cdn/public/"+upload.Key, urls[upload.FileID])
}

func TestResolvePublicURLsSkipsPendingFiles(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProviderProfilePhoto,
	})
	require.NoError(t, err)

	urls, err := service.ResolvePublicURLs(context.Background(), []string{upload.FileID})

	require.NoError(t, err)
	assert.NotContains(t, urls, upload.FileID)
}

func TestResolvePublicURLsReturnsEmptyMapWhenNoFileIDsAreProvided(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	urls, err := service.ResolvePublicURLs(context.Background(), []string{"", ""})

	require.NoError(t, err)
	assert.Empty(t, urls)
}

func TestResolvePublicURLsWrapsRepositoryError(t *testing.T) {
	expectedErr := errors.New("repository unavailable")
	repo := newFileRepositoryMock()
	repo.findByIDsErr = expectedErr
	service := newFileService(repo, newStorageMock())

	urls, err := service.ResolvePublicURLs(context.Background(), []string{"file-id"})

	assert.Nil(t, urls)
	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "finding public files")
}
