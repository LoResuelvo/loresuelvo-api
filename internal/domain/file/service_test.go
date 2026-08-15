package file_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
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
	dataByObject     map[string][]byte
	generateErr      error
	readErr          error
	objectReadErr    error
}

type audioMetadataParserMock struct {
	metadata filedomain.AudioMetadata
	err      error
}

func (parser *audioMetadataParserMock) Parse(_ []byte) (filedomain.AudioMetadata, error) {
	if parser.err != nil {
		return filedomain.AudioMetadata{}, parser.err
	}
	return parser.metadata, nil
}

func (storage *storageMock) GenerateDownloadURL(_ context.Context, object filedomain.ObjectToDownload) (string, error) {
	return "https://download/" + object.Bucket + "/" + object.Key, nil
}

func newStorageMock() *storageMock {
	return &storageMock{
		metadataByObject: map[string]filedomain.ObjectMetadata{},
		dataByObject:     map[string][]byte{},
	}
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

func (storage *storageMock) ReadObject(_ context.Context, object filedomain.ObjectToDownload) ([]byte, error) {
	if storage.objectReadErr != nil {
		return nil, storage.objectReadErr
	}
	metadata, ok := storage.metadataByObject[object.Bucket+"/"+object.Key]
	if !ok {
		return nil, assert.AnError
	}
	if data, ok := storage.dataByObject[object.Bucket+"/"+object.Key]; ok {
		if object.MaxSizeBytes > 0 && len(data) > object.MaxSizeBytes {
			return nil, assert.AnError
		}
		return append([]byte(nil), data...), nil
	}
	data := make([]byte, metadata.SizeBytes)
	if object.MaxSizeBytes > 0 && len(data) > object.MaxSizeBytes {
		return nil, assert.AnError
	}
	return data, nil
}

func (storage *storageMock) PublicURL(bucket, key string) string {
	return "https://cdn/" + bucket + "/" + key
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
}

func newFileService(repo *fileRepositoryMock, storage *storageMock) *filedomain.Service {
	return newFileServiceWithParser(repo, storage, &audioMetadataParserMock{
		metadata: filedomain.AudioMetadata{DurationSeconds: 18, Codec: "opus"},
	})
}

func newFileServiceWithParser(repo *fileRepositoryMock, storage *storageMock, parser filedomain.AudioMetadataParser) *filedomain.Service {
	return filedomain.NewService(repo, storage, "public", "private", fixedClock{}, parser)
}

func TestRequestUploadCreatesPendingProfilePhoto(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())

	result, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProfilePhoto,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.FileID)
	assert.Contains(t, result.Key, "files/2026/06/profile_photo/")
	assert.Equal(t, "https://upload", result.URL)
	createdFile := repo.files[result.FileID]
	assert.Equal(t, filedomain.StatusPending, createdFile.Status)
	assert.Equal(t, filedomain.VisibilityPublic, createdFile.Visibility)
	assert.Equal(t, "auth0|provider", createdFile.UploadedByAuthID)
}

func TestRequestUploadCreatesPrivateConversationMessageImage(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())

	result, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID: "auth0|consumer", OriginalName: "problem.webp", MimeType: "image/webp", SizeBytes: 1024, Purpose: filedomain.PurposeConversationMessageImage,
	})

	require.NoError(t, err)
	createdFile := repo.files[result.FileID]
	assert.Equal(t, filedomain.VisibilityPrivate, createdFile.Visibility)
	assert.Contains(t, result.Key, "/conversation_message_image/")
}

func TestValidateAndResolveConversationMessageImage(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID: "auth0|consumer", OriginalName: "problem.jpg", MimeType: "image/jpeg", SizeBytes: 1024, Purpose: filedomain.PurposeConversationMessageImage,
	})
	require.NoError(t, err)
	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{AuthID: "auth0|consumer", FileID: upload.FileID, Key: upload.Key, MimeType: "image/jpeg", SizeBytes: 1024})
	require.NoError(t, err)

	validated, err := service.PrepareMessageImages(context.Background(), "auth0|consumer", []string{upload.FileID})
	require.NoError(t, err)
	require.Len(t, validated, 1)
	resolved, err := service.ResolveMessageImages(context.Background(), []string{upload.FileID})
	require.NoError(t, err)
	assert.Equal(t, "problem.jpg", resolved[upload.FileID].OriginalName)
	assert.Equal(t, "https://download/private/"+upload.Key, resolved[upload.FileID].URL)
}

func TestPrepareChatbotMessageImagesReturnsPrivateImageContent(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID: "auth0|consumer", OriginalName: "problem.jpg", MimeType: "image/jpeg", SizeBytes: 1024, Purpose: filedomain.PurposeConversationMessageImage,
	})
	require.NoError(t, err)
	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{AuthID: "auth0|consumer", FileID: upload.FileID, Key: upload.Key, MimeType: "image/jpeg", SizeBytes: 1024})
	require.NoError(t, err)

	prepared, err := service.PrepareChatbotMessageImages(context.Background(), "auth0|consumer", []string{upload.FileID})

	require.NoError(t, err)
	require.Len(t, prepared, 1)
	assert.Equal(t, upload.FileID, prepared[0].FileID)
	assert.Equal(t, "problem.jpg", prepared[0].OriginalName)
	assert.Equal(t, "image/jpeg", prepared[0].MimeType)
	assert.Len(t, prepared[0].Data, 1024)
	assert.Equal(t, "https://download/private/"+upload.Key, prepared[0].URL)
}

func TestPrepareConversationMessageImageRejectsWrongOwnerAndPurpose(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID: "auth0|provider", OriginalName: "profile.jpg", MimeType: "image/jpeg", SizeBytes: 1024, Purpose: filedomain.PurposeProfilePhoto,
	})
	require.NoError(t, err)
	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{AuthID: "auth0|provider", FileID: upload.FileID, Key: upload.Key, MimeType: "image/jpeg", SizeBytes: 1024})
	require.NoError(t, err)

	_, err = service.PrepareMessageImages(context.Background(), "auth0|other", []string{upload.FileID})
	assert.ErrorIs(t, err, filedomain.ErrMessageImageNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
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
		Purpose:      filedomain.PurposeProfilePhoto,
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
		Purpose:      filedomain.PurposeProfilePhoto,
	})

	assert.ErrorIs(t, err, expectedErr)
	assert.ErrorContains(t, err, "saving pending file")
}

func TestRequestUploadAcceptsWebPProfilePhoto(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.webp",
		MimeType:     "image/webp",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProfilePhoto,
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
		Purpose:      filedomain.PurposeProfilePhoto,
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedProfilePhoto)
}

func TestRequestUploadRequiresOriginalName(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:    "auth0|provider",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
		Purpose:   filedomain.PurposeProfilePhoto,
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
		Purpose:      filedomain.PurposeProfilePhoto,
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedProfilePhoto)
}

func TestRequestUploadRejectsOversizedJobRequestImage(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|consumer",
		OriginalName: "problema.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    5*1024*1024 + 1,
		Purpose:      filedomain.PurposeJobRequestImage,
	})

	assert.ErrorIs(t, err, filedomain.ErrJobRequestImageNotAvailable)
}

func TestRequestUploadRejectsInvalidJobRequestImageMimeType(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	_, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|consumer",
		OriginalName: "problema.gif",
		MimeType:     "image/gif",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeJobRequestImage,
	})

	assert.ErrorIs(t, err, filedomain.ErrJobRequestImageNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
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
	assert.NoError(t, service.ValidateProfilePhoto(context.Background(), "auth0|provider", upload.FileID))
}

func TestConfirmUploadConfirmsAudioFromValidatedMetadata(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	audioData := testWebMOpusAudio(18)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|consumer",
		OriginalName: "ruido-bomba.webm",
		MimeType:     "audio/webm",
		SizeBytes:    len(audioData),
		Purpose:      filedomain.PurposeConversationMessageAudio,
	})
	require.NoError(t, err)
	storage.dataByObject["private/"+upload.Key] = audioData

	confirmed, err := service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|consumer",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "audio/webm",
		SizeBytes: len(audioData),
	})

	require.NoError(t, err)
	assert.Equal(t, upload.FileID, confirmed.FileID)
	assert.Equal(t, "audio/webm", confirmed.MimeType)
	assert.Equal(t, "opus", confirmed.Codec)
	assert.Equal(t, 18, confirmed.DurationSeconds)
	assert.True(t, repo.files[upload.FileID].IsConfirmed())
}

func TestConfirmUploadRejectsAudioWithInvalidMetadata(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileServiceWithParser(repo, storage, &audioMetadataParserMock{err: filedomain.ErrUnsupportedMessageAudio})
	audioData := []byte("not a WebM file")
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|consumer",
		OriginalName: "ruido-bomba.webm",
		MimeType:     "audio/webm",
		SizeBytes:    len(audioData),
		Purpose:      filedomain.PurposeConversationMessageAudio,
	})
	require.NoError(t, err)
	storage.dataByObject["private/"+upload.Key] = audioData

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|consumer",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "audio/webm",
		SizeBytes: len(audioData),
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedMessageAudio)
	assert.False(t, repo.files[upload.FileID].IsConfirmed())
}

func TestConfirmUploadRejectsAudioOverDurationLimit(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileServiceWithParser(repo, storage, &audioMetadataParserMock{
		metadata: filedomain.AudioMetadata{DurationSeconds: 301, Codec: "opus"},
	})
	audioData := testWebMOpusAudio(301)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|consumer",
		OriginalName: "audio-extenso.webm",
		MimeType:     "audio/webm",
		SizeBytes:    len(audioData),
		Purpose:      filedomain.PurposeConversationMessageAudio,
	})
	require.NoError(t, err)
	storage.dataByObject["private/"+upload.Key] = audioData

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|consumer",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "audio/webm",
		SizeBytes: len(audioData),
	})

	assert.ErrorIs(t, err, filedomain.ErrUnsupportedMessageAudio)
	assert.False(t, repo.files[upload.FileID].IsConfirmed())
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

	assert.ErrorIs(t, err, filedomain.ErrFileNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
	})
	require.NoError(t, err)

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|other",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/png",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, filedomain.ErrFileNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
	})
	require.NoError(t, err)

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    "auth0|provider",
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  "image/jpeg",
		SizeBytes: 2048,
	})

	assert.ErrorIs(t, err, filedomain.ErrFileNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
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

	assert.ErrorIs(t, err, filedomain.ErrFileNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
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

	assert.ErrorIs(t, err, filedomain.ErrFileNotAvailable)
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
		Purpose:      filedomain.PurposeProfilePhoto,
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

func TestValidateProfilePhotoRequiresConfirmedOwnerFile(t *testing.T) {
	repo := newFileRepositoryMock()
	service := newFileService(repo, newStorageMock())
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProfilePhoto,
	})
	require.NoError(t, err)

	assert.ErrorIs(t, service.ValidateProfilePhoto(context.Background(), "auth0|provider", ""), filedomain.ErrProfilePhotoNotAvailable)
	assert.ErrorIs(t, service.ValidateProfilePhoto(context.Background(), "auth0|provider", upload.FileID), filedomain.ErrProfilePhotoNotAvailable)
}

func TestValidateProfilePhotoRejectsWrongOwner(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProfilePhoto,
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

	err = service.ValidateProfilePhoto(context.Background(), "auth0|other", upload.FileID)

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestValidateProfilePhotoRejectsMissingFile(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	err := service.ValidateProfilePhoto(context.Background(), "auth0|provider", "missing")

	assert.ErrorIs(t, err, filedomain.ErrProfilePhotoNotAvailable)
}

func TestResolvePublicURLReturnsConfirmedPublicFileURL(t *testing.T) {
	repo := newFileRepositoryMock()
	storage := newStorageMock()
	service := newFileService(repo, storage)
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       "auth0|provider",
		OriginalName: "foto.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    1024,
		Purpose:      filedomain.PurposeProfilePhoto,
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

	url, err := service.ResolvePublicURL(context.Background(), upload.FileID)

	require.NoError(t, err)
	assert.Equal(t, "https://cdn/public/"+upload.Key, url)
}

func TestResolvePublicURLReturnsEmptyURLForUnavailablePublicFile(t *testing.T) {
	service := newFileService(newFileRepositoryMock(), newStorageMock())

	url, err := service.ResolvePublicURL(context.Background(), "")

	require.NoError(t, err)
	assert.Empty(t, url)
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
		Purpose:      filedomain.PurposeProfilePhoto,
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
		Purpose:      filedomain.PurposeProfilePhoto,
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

func testWebMOpusAudio(durationSeconds int) []byte {
	duration := make([]byte, 8)
	binary.BigEndian.PutUint64(duration, math.Float64bits(float64(durationSeconds)*1000))

	ebmlHeader := testEBMLElement(0x1A45DFA3,
		testEBMLElement(0x4286, []byte{1}),
		testEBMLElement(0x4282, []byte("webm")),
	)
	info := testEBMLElement(0x1549A966,
		testEBMLElement(0x2AD7B1, []byte{0x0F, 0x42, 0x40}),
		testEBMLElement(0x4489, duration),
	)
	tracks := testEBMLElement(0x1654AE6B, testEBMLElement(0xAE, testEBMLElement(0x86, []byte("A_OPUS"))))
	segment := testEBMLElement(0x18538067, append(info, tracks...))
	return append(ebmlHeader, segment...)
}

func testEBMLElement(id uint64, payload ...[]byte) []byte {
	var body []byte
	for _, part := range payload {
		body = append(body, part...)
	}
	idBytes := testEBMLID(id)
	result := make([]byte, 0, len(idBytes)+8+len(body))
	result = append(result, idBytes...)
	result = append(result, testEBMLSize(len(body))...)
	result = append(result, body...)
	return result
}

func testEBMLID(id uint64) []byte {
	length := 1
	for value := id; value > 0xff; value >>= 8 {
		length++
	}
	result := make([]byte, length)
	for index := length - 1; index >= 0; index-- {
		result[index] = byte(id)
		id >>= 8
	}
	return result
}

func testEBMLSize(size int) []byte {
	for length := 1; length <= 8; length++ {
		max := (uint64(1) << uint(7*length)) - 2
		if uint64(size) > max {
			continue
		}
		result := make([]byte, length)
		value := uint64(size)
		for index := length - 1; index >= 0; index-- {
			result[index] = byte(value)
			value >>= 8
		}
		result[0] |= byte(1 << uint(8-length))
		return result
	}
	panic("test WebM element payload is too large")
}
