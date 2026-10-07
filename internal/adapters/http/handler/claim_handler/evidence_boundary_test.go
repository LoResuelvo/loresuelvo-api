package claim_handler

import (
	"fmt"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/file_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubmitRealServicesClassifyEvidenceValidationErrors(t *testing.T) {
	for _, mode := range []string{"pending", "other owner", "other purpose", "missing", "database failure"} {
		t.Run(mode, func(t *testing.T) {
			files := new(fileRepositoryMock)
			images := filedomain.NewService(files, nil, "public", "private", clockadapter.NewSystemClock(), nil)
			claims, users, operations := new(claimRepositoryMock), new(claimantFinderMock), new(operationResolverMock)
			claimant := claim.Claimant{ID: 1, Party: claim.PartyConsumer}
			users.On("FindClaimantByAuthID", mock.Anything, "subject").Return(&claimant, nil).Once()
			claims.On("FindBySubmissionKey", mock.Anything, 1, mock.Anything).Return((*claim.Claim)(nil), nil).Once()
			operations.On("ResolveClaimOperationReference", mock.Anything, claimant, mock.Anything).Return(&operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 1}, nil).Once()
			imageID := uuid.NewString()
			metadata, err := filedomain.NewFileMetadata("image.png", "image/png", 32)
			require.NoError(t, err)
			f, err := filedomain.NewPendingFile(imageID, "image", "private", metadata, filedomain.VisibilityPrivate, filedomain.PurposeClaimEvidenceImage, "subject", time.Now())
			require.NoError(t, err)
			f.Confirm(time.Now())
			switch mode {
			case "pending":
				f.Status = filedomain.StatusPending
			case "other owner":
				f.UploadedByAuthID = "other"
			case "other purpose":
				f.Purpose = filedomain.PurposeConversationMessageImage
			}
			found := []filedomain.File{*f}
			var findErr error
			if mode == "missing" {
				found = nil
			}
			if mode == "database failure" {
				findErr = fmt.Errorf("private database failure")
			}
			files.On("FindByIDs", mock.Anything, []string{imageID}).Return(found, findErr).Once()
			svc := claim.NewService(claims, users, operations, images, clockadapter.NewSystemClock())
			response := perform(svc, "POST", "/claims", fmt.Sprintf(`{"reference":{"job_request_id":1},"reason":"damage","description":"x","image_file_ids":[%q]}`, imageID), true)
			expected := 400
			if findErr != nil {
				expected = 500
			}
			require.Equal(t, expected, response.Code)
			require.NotContains(t, response.Body.String(), "private database")
			claims.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
			for _, m := range []*mock.Mock{&files.Mock, &claims.Mock, &users.Mock, &operations.Mock} {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestFileEndpointsRealServiceClassifiesClaimEvidenceErrors(t *testing.T) {
	for _, mode := range []string{"invalid MIME", "invalid size", "invalid body", "metadata failure", "body failure", "save failure", "lookup failure", "missing file"} {
		t.Run(mode, func(t *testing.T) {
			files, objects := new(fileRepositoryMock), new(storageMock)
			svc := filedomain.NewService(files, objects, "public", "private", clockadapter.NewSystemClock(), nil)
			h := file_handler.NewFileHandler(svc)
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "subject") })
			r.POST("/files/presign", h.PresignUpload)
			r.POST("/files/:fileID/confirm", h.ConfirmUpload)
			path, body := "/files/presign", `{"original_name":"image.gif","mime_type":"image/gif","size_bytes":32,"purpose":"claim_evidence_image"}`
			if mode == "invalid size" {
				body = `{"original_name":"image.png","mime_type":"image/png","size_bytes":5242881,"purpose":"claim_evidence_image"}`
			}
			if mode == "save failure" {
				body = `{"original_name":"image.png","mime_type":"image/png","size_bytes":32,"purpose":"claim_evidence_image"}`
				objects.On("GenerateUploadURL", mock.Anything, mock.Anything).Return(&filedomain.UploadTarget{URL: "https://upload"}, nil).Once()
				files.On("Save", mock.Anything, mock.Anything).Return(fmt.Errorf("private database failure")).Once()
			}
			if mode == "lookup failure" || mode == "missing file" {
				path = "/files/" + uuid.NewString() + "/confirm"
				body = `{"key":"image","mime_type":"image/png","size_bytes":32}`
				lookupErr := fmt.Errorf("private database failure")
				if mode == "missing file" {
					lookupErr = filedomain.ErrFileNotAvailable
				}
				files.On("FindByID", mock.Anything, mock.Anything).Return((*filedomain.File)(nil), lookupErr).Once()
			}
			if mode == "invalid body" || mode == "metadata failure" || mode == "body failure" {
				metadata, err := filedomain.NewFileMetadata("image.png", "image/png", 32)
				require.NoError(t, err)
				f, err := filedomain.NewPendingFile(uuid.NewString(), "image", "private", metadata, filedomain.VisibilityPrivate, filedomain.PurposeClaimEvidenceImage, "subject", time.Now())
				require.NoError(t, err)
				path = "/files/" + f.ID + "/confirm"
				body = `{"key":"image","mime_type":"image/png","size_bytes":32}`
				files.On("FindByID", mock.Anything, f.ID).Return(f, nil).Once()
				var metadataErr, errorBody error
				if mode == "metadata failure" {
					metadataErr = fmt.Errorf("private storage failure")
				}
				objects.On("ReadObjectMetadata", mock.Anything, "private", "image").Return(&filedomain.ObjectMetadata{MimeType: "image/png", SizeBytes: 32}, metadataErr).Once()
				if metadataErr == nil {
					if mode == "body failure" {
						errorBody = fmt.Errorf("private storage failure")
					}
					objects.On("ReadObject", mock.Anything, mock.Anything).Return(make([]byte, 32), errorBody).Once()
				}
			}
			req := httptest.NewRequest("POST", path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			r.ServeHTTP(response, req)
			expected := 400
			if strings.Contains(mode, "failure") {
				expected = 500
			}
			require.Equal(t, expected, response.Code)
			require.NotContains(t, response.Body.String(), "private storage")
			require.NotContains(t, response.Body.String(), "private database")
			files.AssertExpectations(t)
			objects.AssertExpectations(t)
		})
	}
}

func TestDetailRealServicesFailClosedOnUnavailableAndTechnicalEvidence(t *testing.T) {
	for _, mode := range []string{"pending", "missing metadata", "foreign owner", "wrong purpose", "database failure", "storage failure"} {
		t.Run(mode, func(t *testing.T) {
			files, objects := new(fileRepositoryMock), new(storageMock)
			images := filedomain.NewService(files, objects, "public", "private", clockadapter.NewSystemClock(), nil)
			claims, users := new(claimRepositoryMock), new(claimantFinderMock)
			users.On("FindClaimantByAuthID", mock.Anything, "subject").Return(&claim.Claimant{ID: 1, Party: claim.PartyConsumer}, nil).Once()
			imageID := uuid.NewString()
			claims.On("FindOwnedByID", mock.Anything, 1, 3).Return(&claim.Claim{ID: 3, ImageFileIDs: []string{imageID}}, nil).Once()
			metadata, err := filedomain.NewFileMetadata("image.png", "image/png", 32)
			require.NoError(t, err)
			f, err := filedomain.NewPendingFile(imageID, "image", "private", metadata, filedomain.VisibilityPrivate, filedomain.PurposeClaimEvidenceImage, "subject", time.Now())
			require.NoError(t, err)
			var findErr error
			if mode == "database failure" {
				findErr = fmt.Errorf("private database failure")
			}
			if mode != "pending" {
				f.Confirm(time.Now())
			}
			switch mode {
			case "foreign owner":
				f.UploadedByAuthID = "other"
			case "wrong purpose":
				f.Purpose = filedomain.PurposeConversationMessageImage
			}
			foundFiles := []filedomain.File{*f}
			if mode == "missing metadata" {
				foundFiles = nil
			}
			files.On("FindByIDs", mock.Anything, []string{imageID}).Return(foundFiles, findErr).Once()
			if mode == "storage failure" {
				objects.On("GenerateDownloadURL", mock.Anything, mock.Anything).Return("", fmt.Errorf("private storage failure")).Once()
			}
			svc := claim.NewService(claims, users, nil, images, clockadapter.NewSystemClock())
			response := perform(svc, "GET", "/claims/3", "", true)
			require.Equal(t, 500, response.Code)
			require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
			require.NotContains(t, response.Body.String(), "private database")
			require.NotContains(t, response.Body.String(), "private storage")
			require.NotContains(t, response.Body.String(), "url")
			for _, m := range []*mock.Mock{&claims.Mock, &users.Mock, &files.Mock, &objects.Mock} {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestDetailDiscardsResolvedURLsWhenLaterEvidenceFails(t *testing.T) {
	files, objects := new(fileRepositoryMock), new(storageMock)
	images := filedomain.NewService(files, objects, "public", "private", clockadapter.NewSystemClock(), nil)
	claims, users := new(claimRepositoryMock), new(claimantFinderMock)
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	users.On("FindClaimantByAuthID", mock.Anything, "subject").Return(&claim.Claimant{ID: 1, Party: claim.PartyConsumer}, nil).Once()
	claims.On("FindOwnedByID", mock.Anything, 1, 3).Return(&claim.Claim{ID: 3, Description: "Private testimony", ImageFileIDs: ids}, nil).Once()
	for i, id := range ids {
		metadata, err := filedomain.NewFileMetadata("image.png", "image/png", 32)
		require.NoError(t, err)
		f, err := filedomain.NewPendingFile(id, id, "private", metadata, filedomain.VisibilityPrivate, filedomain.PurposeClaimEvidenceImage, "subject", time.Now())
		require.NoError(t, err)
		f.Confirm(time.Now())
		files.On("FindByIDs", mock.Anything, []string{id}).Return([]filedomain.File{*f}, nil).Once()
		target := filedomain.ObjectToDownload{Bucket: "private", Key: id}
		if i == len(ids)-1 {
			objects.On("GenerateDownloadURL", mock.Anything, target).Return("", fmt.Errorf("private storage failure")).Once()
		} else {
			objects.On("GenerateDownloadURL", mock.Anything, target).Return("https://private/"+id+"?signature=temporary", nil).Once()
		}
	}
	svc := claim.NewService(claims, users, nil, images, clockadapter.NewSystemClock())
	response := perform(svc, "GET", "/claims/3", "", true)
	require.Equal(t, 500, response.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
	for _, m := range []*mock.Mock{&claims.Mock, &users.Mock, &files.Mock, &objects.Mock} {
		m.AssertExpectations(t)
	}
}
