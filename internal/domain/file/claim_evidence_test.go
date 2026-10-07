package file_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimEvidencePolicyAndConfirmation(t *testing.T) {
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	var jpegEncoded bytes.Buffer
	require.NoError(t, jpeg.Encode(&jpegEncoded, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil))
	for _, tc := range []struct {
		name, mime string
		data       []byte
		valid      bool
	}{
		{"valid PNG", "image/png", encoded.Bytes(), true},
		{"valid JPEG", "image/jpeg", jpegEncoded.Bytes(), true},
		{"false MIME", "image/jpeg", encoded.Bytes(), false},
		{"invalid bytes", "image/png", []byte("not an image"), false},
		{"truncated PNG", "image/png", encoded.Bytes()[:16], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, storage := newFileRepositoryMock(), newStorageMock()
			svc := newFileService(repo, storage)
			upload, err := svc.RequestUpload(context.Background(), filedomain.PresignRequest{AuthID: "owner", OriginalName: "image", MimeType: tc.mime, SizeBytes: len(tc.data), Purpose: filedomain.PurposeClaimEvidenceImage})
			require.NoError(t, err)
			require.True(t, storage.lastUpload.CreateOnly)
			storage.dataByObject["private/"+upload.Key] = tc.data
			_, err = svc.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{AuthID: "owner", FileID: upload.FileID, Key: upload.Key, MimeType: tc.mime, SizeBytes: len(tc.data)})
			if !tc.valid {
				require.ErrorIs(t, err, filedomain.ErrClaimEvidenceImageNotAvailable)
				require.False(t, repo.files[upload.FileID].IsConfirmed())
				return
			}
			require.NoError(t, err)
			require.NoError(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", []string{upload.FileID}))
			require.ErrorIs(t, svc.ValidateClaimEvidenceImages(context.Background(), "other", []string{upload.FileID}), filedomain.ErrClaimEvidenceImageNotAvailable)
			require.ErrorIs(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", []string{upload.FileID, upload.FileID}), filedomain.ErrClaimEvidenceImageNotAvailable)
			url, err := svc.ResolveClaimEvidenceImage(context.Background(), "owner", upload.FileID)
			require.NoError(t, err)
			require.Contains(t, url, "https://download/private/")
			storage.downloadErr = assert.AnError
			url, err = svc.ResolveClaimEvidenceImage(context.Background(), "owner", upload.FileID)
			require.ErrorIs(t, err, assert.AnError)
			require.Empty(t, url)
		})
	}
}

func TestClaimEvidenceRejectsInvalidMetadataAndIDs(t *testing.T) {
	svc := newFileService(newFileRepositoryMock(), newStorageMock())
	for _, ids := range [][]string{{""}, {"not-uuid"}, {"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", "00000000-0000-0000-0000-000000000003", "00000000-0000-0000-0000-000000000004"}} {
		require.ErrorIs(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", ids), filedomain.ErrClaimEvidenceImageNotAvailable)
	}
	require.NoError(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", nil))
	for _, tc := range []struct {
		mime string
		size int
	}{{"image/webp", 12}, {"image/gif", 12}, {"image/png", 5*1024*1024 + 1}} {
		_, err := svc.RequestUpload(context.Background(), filedomain.PresignRequest{AuthID: "owner", OriginalName: "image", MimeType: tc.mime, SizeBytes: tc.size, Purpose: filedomain.PurposeClaimEvidenceImage})
		require.ErrorIs(t, err, filedomain.ErrClaimEvidenceImageNotAvailable)
	}
}

func TestClaimEvidenceRequiresConfirmedPrivateOwnedPurposeFile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*filedomain.File)
	}{
		{"pending", func(f *filedomain.File) { f.Status = filedomain.StatusPending }},
		{"public", func(f *filedomain.File) { f.Visibility = filedomain.VisibilityPublic }},
		{"other purpose", func(f *filedomain.File) { f.Purpose = filedomain.PurposeConversationMessageImage }},
		{"other owner", func(f *filedomain.File) { f.UploadedByAuthID = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, storage := newFileRepositoryMock(), newStorageMock()
			svc := newFileService(repo, storage)
			result, err := svc.RequestUpload(context.Background(), filedomain.PresignRequest{AuthID: "owner", OriginalName: "image.png", MimeType: "image/png", SizeBytes: 32, Purpose: filedomain.PurposeClaimEvidenceImage})
			require.NoError(t, err)
			f := repo.files[result.FileID]
			f.Confirm(f.CreatedOn)
			tc.mutate(&f)
			repo.files[f.ID] = f
			require.ErrorIs(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", []string{f.ID}), filedomain.ErrClaimEvidenceImageNotAvailable)
			url, err := svc.ResolveClaimEvidenceImage(context.Background(), "owner", f.ID)
			require.Error(t, err)
			require.Empty(t, url)
		})
	}
}

func TestClaimEvidenceStorageFailuresDoNotConfirm(t *testing.T) {
	for _, failure := range []string{"metadata", "body", "size"} {
		t.Run(failure, func(t *testing.T) {
			repo, storage := newFileRepositoryMock(), newStorageMock()
			svc := newFileService(repo, storage)
			result, err := svc.RequestUpload(context.Background(), filedomain.PresignRequest{AuthID: "owner", OriginalName: "image.png", MimeType: "image/png", SizeBytes: 32, Purpose: filedomain.PurposeClaimEvidenceImage})
			require.NoError(t, err)
			switch failure {
			case "metadata":
				storage.readErr = assert.AnError
			case "body":
				storage.objectReadErr = assert.AnError
			case "size":
				storage.dataByObject["private/"+result.Key] = []byte("bad")
			}
			_, err = svc.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{AuthID: "owner", FileID: result.FileID, Key: result.Key, MimeType: "image/png", SizeBytes: 32})
			if failure != "size" {
				require.ErrorIs(t, err, assert.AnError)
			} else {
				require.ErrorIs(t, err, filedomain.ErrClaimEvidenceImageNotAvailable)
			}
			require.False(t, repo.files[result.FileID].IsConfirmed())
		})
	}
}

func TestClaimEvidenceValidationDoesNotReadStorage(t *testing.T) {
	repo, storage := newFileRepositoryMock(), newStorageMock()
	svc := newFileService(repo, storage)
	result, err := svc.RequestUpload(context.Background(), filedomain.PresignRequest{AuthID: "owner", OriginalName: "image.png", MimeType: "image/png", SizeBytes: 32, Purpose: filedomain.PurposeClaimEvidenceImage})
	require.NoError(t, err)
	f := repo.files[result.FileID]
	f.Confirm(f.CreatedOn)
	repo.files[f.ID] = f
	storage.readErr = assert.AnError
	storage.objectReadErr = assert.AnError
	storage.generateErr = assert.AnError
	require.NoError(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", []string{f.ID}))
	repo.findByIDsErr = assert.AnError
	require.ErrorIs(t, svc.ValidateClaimEvidenceImages(context.Background(), "owner", []string{f.ID}), assert.AnError)
}
