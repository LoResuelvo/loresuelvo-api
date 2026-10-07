package file_test

import (
	"context"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
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
		return nil, filedomain.ErrFileNotAvailable
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
	downloadErr      error
	lastUpload       filedomain.ObjectToUpload
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

type videoMetadataParserMock struct {
	metadata filedomain.VideoMetadata
	err      error
}

func (parser *videoMetadataParserMock) Parse(_ []byte) (filedomain.VideoMetadata, error) {
	if parser.err != nil {
		return filedomain.VideoMetadata{}, parser.err
	}
	return parser.metadata, nil
}

func (storage *storageMock) GenerateDownloadURL(_ context.Context, object filedomain.ObjectToDownload) (string, error) {
	if storage.downloadErr != nil {
		return "", storage.downloadErr
	}
	return "https://download/" + object.Bucket + "/" + object.Key, nil
}

func newStorageMock() *storageMock {
	return &storageMock{
		metadataByObject: map[string]filedomain.ObjectMetadata{},
		dataByObject:     map[string][]byte{},
	}
}

func (storage *storageMock) GenerateUploadURL(_ context.Context, object filedomain.ObjectToUpload) (*filedomain.UploadTarget, error) {
	storage.lastUpload = object
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
	return filedomain.NewService(repo, storage, "public", "private", fixedClock{}, parser, nil)
}

func newFileServiceWithVideoParser(repo *fileRepositoryMock, storage *storageMock, parser filedomain.VideoMetadataParser) *filedomain.Service {
	return filedomain.NewService(repo, storage, "public", "private", fixedClock{}, &audioMetadataParserMock{
		metadata: filedomain.AudioMetadata{DurationSeconds: 18, Codec: "opus"},
	}, parser)
}

func createWorkOrderCompletionImage(
	t *testing.T,
	service *filedomain.Service,
	authID string,
	purpose string,
	fileName string,
	mimeType string,
	sizeBytes int,
	confirm bool,
) string {
	t.Helper()
	upload, err := service.RequestUpload(context.Background(), filedomain.PresignRequest{
		AuthID:       authID,
		OriginalName: fileName,
		MimeType:     mimeType,
		SizeBytes:    sizeBytes,
		Purpose:      purpose,
	})
	require.NoError(t, err)
	if !confirm {
		return upload.FileID
	}

	_, err = service.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{
		AuthID:    authID,
		FileID:    upload.FileID,
		Key:       upload.Key,
		MimeType:  mimeType,
		SizeBytes: sizeBytes,
	})
	require.NoError(t, err)
	return upload.FileID
}
