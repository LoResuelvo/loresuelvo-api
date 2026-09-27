package operation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mediaAssociationStub struct {
	purpose string
	err     error
	calls   int
	gotID   readmodel.ID
}

func (stub *mediaAssociationStub) FindAssociatedImagePurpose(_ context.Context, id readmodel.ID, _ string) (string, error) {
	stub.calls++
	stub.gotID = id
	return stub.purpose, stub.err
}

type mediaFileStub struct {
	file  *filedomain.File
	err   error
	calls int
}

func (stub *mediaFileStub) FindByID(context.Context, string) (*filedomain.File, error) {
	stub.calls++
	return stub.file, stub.err
}

type mediaStorageStub struct {
	data   []byte
	err    error
	calls  int
	object filedomain.ObjectToDownload
}

func (stub *mediaStorageStub) ReadObject(_ context.Context, object filedomain.ObjectToDownload) ([]byte, error) {
	stub.calls++
	stub.object = object
	return stub.data, stub.err
}

func testOperationImage(t *testing.T, purpose string, size int) *filedomain.File {
	t.Helper()
	metadata, err := filedomain.NewFileMetadata("photo.jpg", "image/jpeg", size)
	require.NoError(t, err)
	file, err := filedomain.NewFile(uuid.NewString(), "object-key", "private-bucket", metadata,
		filedomain.StatusConfirmed, filedomain.VisibilityPrivate, purpose, "uploader", time.Now(), time.Now())
	require.NoError(t, err)
	return file
}

func TestMediaServiceReadsOnlyAssociatedPrivateImage(t *testing.T) {
	file := testOperationImage(t, filedomain.PurposeWorkOrderCompletionImage, 3)
	association := &mediaAssociationStub{purpose: filedomain.PurposeWorkOrderCompletionImage}
	files := &mediaFileStub{file: file}
	storage := &mediaStorageStub{data: []byte("abc")}
	image, err := operation.NewMediaService(association, files, storage).Get(context.Background(), "sp-42", file.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("abc"), image.Bytes)
	require.Equal(t, "image/jpeg", image.MimeType)
	require.Equal(t, readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 42}, association.gotID)
	require.Equal(t, filedomain.ObjectToDownload{Bucket: file.Bucket, Key: file.Key, MaxSizeBytes: 3}, storage.object)
}

func TestMediaServiceRejectsInvalidAndUnassociatedImagesBeforeStorage(t *testing.T) {
	file := testOperationImage(t, filedomain.PurposeJobRequestImage, 3)
	association := &mediaAssociationStub{}
	files := &mediaFileStub{file: file}
	storage := &mediaStorageStub{data: []byte("abc")}
	service := operation.NewMediaService(association, files, storage)
	_, err := service.Get(context.Background(), "jr-0", file.ID)
	require.ErrorIs(t, err, operation.ErrInvalidOperationID)
	_, err = service.Get(context.Background(), "jr-1", "not-a-uuid")
	require.ErrorIs(t, err, operation.ErrInvalidOperationImageID)
	require.Zero(t, association.calls)
	_, err = service.Get(context.Background(), "jr-1", file.ID)
	require.ErrorIs(t, err, operation.ErrOperationImageNotFound)
	require.Zero(t, files.calls)
	require.Zero(t, storage.calls)
}

func TestMediaServiceRejectsMismatchedOrUnavailableFiles(t *testing.T) {
	file := testOperationImage(t, filedomain.PurposeJobRequestImage, 3)
	association := &mediaAssociationStub{purpose: filedomain.PurposeJobRequestImage}
	files := &mediaFileStub{file: file}
	storage := &mediaStorageStub{data: []byte("abc")}
	service := operation.NewMediaService(association, files, storage)

	files.file = nil
	_, err := service.Get(context.Background(), "jr-1", file.ID)
	require.ErrorIs(t, err, operation.ErrOperationImageNotFound)
	files.file = file
	file.Visibility = filedomain.VisibilityPublic
	_, err = service.Get(context.Background(), "jr-1", file.ID)
	require.ErrorIs(t, err, operation.ErrOperationImageNotFound)
	file.Visibility = filedomain.VisibilityPrivate
	file.Purpose = filedomain.PurposeProfilePhoto
	_, err = service.Get(context.Background(), "jr-1", file.ID)
	require.ErrorIs(t, err, operation.ErrOperationImageNotFound)
	file.Purpose = filedomain.PurposeJobRequestImage
	file.Status = filedomain.StatusPending
	_, err = service.Get(context.Background(), "jr-1", file.ID)
	require.ErrorIs(t, err, operation.ErrOperationImageNotFound)
	require.Zero(t, storage.calls)
}

func TestMediaServiceRejectsStorageFailureAndLengthMismatch(t *testing.T) {
	file := testOperationImage(t, filedomain.PurposeJobRequestImage, 3)
	storage := &mediaStorageStub{data: []byte("too long")}
	service := operation.NewMediaService(&mediaAssociationStub{purpose: file.Purpose}, &mediaFileStub{file: file}, storage)
	_, err := service.Get(context.Background(), "jr-1", file.ID)
	require.Error(t, err)
	require.NotErrorIs(t, err, operation.ErrOperationImageNotFound)
	storage.err = errors.New("storage down")
	_, err = service.Get(context.Background(), "jr-1", file.ID)
	require.ErrorIs(t, err, storage.err)
}
