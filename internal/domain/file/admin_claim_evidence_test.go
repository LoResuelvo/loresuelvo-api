package file_test

import (
	"context"
	"errors"
	"testing"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdministrativeClaimEvidenceUsesPrivatePolicyWithoutOwnerImpersonation(t *testing.T) {
	for _, mode := range []string{"valid", "pending", "public", "wrong purpose", "missing", "lookup failure", "signing failure"} {
		t.Run(mode, func(t *testing.T) {
			repo, storage := newFileRepositoryMock(), newStorageMock()
			svc := newFileService(repo, storage)
			metadata, err := filedomain.NewFileMetadata("image.png", "image/png", 32)
			require.NoError(t, err)
			file, err := filedomain.NewPendingFile(uuid.NewString(), "evidence", "private", metadata, filedomain.VisibilityPrivate, filedomain.PurposeClaimEvidenceImage, "claimant-not-admin", time.Now())
			require.NoError(t, err)
			file.Confirm(time.Now())
			switch mode {
			case "pending":
				file.Status = filedomain.StatusPending
			case "public":
				file.Visibility = filedomain.VisibilityPublic
			case "wrong purpose":
				file.Purpose = filedomain.PurposeConversationMessageImage
			case "lookup failure":
				repo.findByIDsErr = errors.New("private database failure")
			case "signing failure":
				storage.downloadErr = errors.New("private storage failure")
			}
			if mode != "missing" {
				repo.files[file.ID] = *file
			}
			url, err := svc.ResolveAdministrativeClaimEvidenceImage(context.Background(), file.ID)
			if mode == "valid" {
				require.NoError(t, err)
				require.NotEmpty(t, url)
			} else {
				require.Error(t, err)
				require.Empty(t, url)
			}
		})
	}
}
