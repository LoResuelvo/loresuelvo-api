package claim_handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func perform(s service, method, path, body string, authenticated bool) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if authenticated {
		r.Use(func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "subject") })
	}
	h := NewHandler(s)
	r.POST("/claims", h.Submit)
	r.GET("/claims", h.List)
	r.GET("/claims/:id", h.Get)
	r.GET("/claims/:id/images/:file_id", h.ResolveImage)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestSubmitRejectsInvalidContractsBeforeService(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"reference":{"job_request_id":1,"job_request_id":2},"reason":"damage","description":"x"}`, `{"reference":{"job_request_id":1},"reason":"damage","reason":"poor_quality","description":"x"}`, `{"reference":{"job_request_id":1,"work_order_id":2},"reason":"damage","description":"x"}`, `{"reference":{"job_request_id":null},"reason":"damage","description":"x"}`, `{"reference":{"job_request_id":"1"},"reason":"damage","description":"x"}`, `{"reference":{"job_request_id":1,"unknown":2},"reason":"damage","description":"x"}`, `{"reference":{"job_request_id":1},"reason":"damage","description":"x","claimant_id":2}`, `{"reference":{"job_request_id":1},"reason":"damage","description":"x","image_file_ids":null}`, `{"reference":{"job_request_id":1},"reason":"damage","description":null}`, `{"reference":{"job_request_id":1},"reason":"damage","description":"x"} {}`, `{"reference":{"job_request_id":1},"reason":"damage","description":"x\u0000y"}`, strings.Repeat(" ", 65537)} {
		t.Run(body[:min(len(body), 80)], func(t *testing.T) {
			s := newServiceMock(t)
			w := perform(s, "POST", "/claims", body, true)
			require.Equal(t, 400, w.Code)
			require.Empty(t, s.Calls)
		})
	}
}
func TestSubmitAcknowledgmentAndReplay(t *testing.T) {
	for _, created := range []bool{true, false} {
		ref, err := claim.NewReference(claim.ReferenceKindJobRequest, "4")
		require.NoError(t, err)
		s := newServiceMock(t)
		s.On("Submit", mock.Anything, "subject", mock.Anything, mock.Anything).Return(&claim.SubmissionResult{Created: created, Claim: &claim.Claim{ID: 7, OperationID: operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 4}, Reference: ref, Status: claim.StatusOpen, CreatedOn: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}, nil).Once()
		w := perform(s, "POST", "/claims", `{"reference":{"job_request_id":4},"reason":"damage","description":"x"}`, true)
		if created {
			require.Equal(t, 201, w.Code)
			require.Equal(t, "/claims/7", w.Header().Get("Location"))
		} else {
			require.Equal(t, 200, w.Code)
		}
		require.JSONEq(t, `{"id":7,"operation_id":"jr-4","reference":{"job_request_id":4},"status":"open","created_on":"2026-01-01T00:00:00Z"}`, w.Body.String())
	}
}
func TestErrorsAreStableAndPrivate(t *testing.T) {
	for _, tc := range []struct {
		err     error
		code    int
		message string
	}{{claim.ErrInvalidSubmission, 400, "invalid claim submission"}, {claim.ErrForbidden, 403, "claimant account is not enabled"}, {claim.ErrNotFound, 404, "claim resource not found"}, {claim.ErrSubmissionKeyConflict, 409, "claim submission key already used with different content"}, {claim.ErrOpenClaimConflict, 409, "an unfinished claim already exists for this operation"}, {errors.New("SQL secret"), 500, "internal server error"}} {
		s := newServiceMock(t)
		s.On("Get", mock.Anything, "subject", 1).Return((*claim.Claim)(nil), tc.err).Once()
		w := perform(s, "GET", "/claims/1", "", true)
		require.Equal(t, tc.code, w.Code)
		require.JSONEq(t, `{"error":"`+tc.message+`"}`, w.Body.String())
	}
}
func TestDetailEmptyResolutionAndCollections(t *testing.T) {
	s := newServiceMock(t)
	s.On("Get", mock.Anything, "subject", mock.Anything).Return(&claim.Claim{ID: 1}, nil).Once()
	w := perform(s, "GET", "/claims/1", "", true)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"resolution":null`)
	require.Contains(t, w.Body.String(), `"image_file_ids":[]`)
	for _, field := range []string{"Actions", "actions", "claimant_id", "operator", "submissionKey"} {
		require.NotContains(t, w.Body.String(), field)
	}
}
func TestListDefaultsAndStrictQuery(t *testing.T) {
	s := newServiceMock(t)
	s.On("List", mock.Anything, "subject", mock.Anything).Return(&claim.Page{Page: 1, Limit: 20}, nil).Once()
	w := perform(s, "GET", "/claims", "", true)
	require.Equal(t, 200, w.Code)
	require.Equal(t, 1, s.Calls[0].Arguments.Get(2).(claim.ListCriteria).Page)
	require.Equal(t, 20, s.Calls[0].Arguments.Get(2).(claim.ListCriteria).Limit)
	require.Contains(t, w.Body.String(), `"items":[]`)
	for _, query := range []string{"page=0", "page=-1", "limit=101", "limit=0", "status=other", "claimant_id=2", "page=1&page=2", "page=", "status="} {
		s := newServiceMock(t)
		w := perform(s, "GET", "/claims?"+query, "", true)
		require.Equal(t, 400, w.Code)
		require.Empty(t, s.Calls)
	}
}
func TestMissingIdentityIsUnauthorized(t *testing.T) {
	s := newServiceMock(t)
	w := perform(s, "GET", "/claims/1", "", false)
	require.Equal(t, 401, w.Code)
	require.JSONEq(t, `{"error":"missing user id"}`, w.Body.String())
	require.Empty(t, s.Calls)
}

func TestDetailFormalResolutionUsesParticipantDTO(t *testing.T) {
	closed := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	s := newServiceMock(t)
	s.On("Get", mock.Anything, "subject", mock.Anything).Return(&claim.Claim{ID: 4, ClaimantID: 99, ClaimantParty: claim.PartyConsumer, Description: "Original testimony", ImageFileIDs: []string{"private-file"}, Actions: []claim.Action{{ActorID: 100}}, Resolution: &claim.Resolution{Type: claim.ResolutionTypeConsumerFavor, Reasoning: "Formal reasoning", ResolvedOn: closed, SuggestedCompensation: &claim.SuggestedCompensation{AmountMinor: 1500, Currency: "ARS", Unit: "minor"}}}, nil).Once()
	w := perform(s, "GET", "/claims/4", "", true)
	require.Equal(t, 200, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, map[string]any{"type": "consumer_favor", "reasoning": "Formal reasoning", "resolved_on": "2026-09-24T12:00:00Z", "suggested_compensation": map[string]any{"amount_minor": float64(1500), "currency": "ARS", "unit": "minor"}}, result["resolution"])
	for _, field := range []string{"claimant_id", "actions", "operator_id", "executed", "transactions", "submission_key"} {
		require.NotContains(t, result, field)
	}
}
func TestImageErrorsDoNotLeakStorage(t *testing.T) {
	for _, err := range []error{claim.ErrNotFound, filedomain.ErrClaimEvidenceImageNotAvailable, errors.New("s3 secret key bucket")} {
		s := newServiceMock(t)
		s.On("ResolveImage", mock.Anything, "subject", 3, "private").Return("", err).Once()
		w := perform(s, "GET", "/claims/3/images/private", "", true)
		if errors.Is(err, claim.ErrNotFound) || errors.Is(err, filedomain.ErrClaimEvidenceImageNotAvailable) {
			require.Equal(t, 404, w.Code)
		} else {
			require.Equal(t, 500, w.Code)
		}
		require.NotContains(t, w.Body.String(), "url")
		require.NotContains(t, w.Body.String(), "s3 secret")
	}
}
func TestSubmissionInvalidEvidenceAndInternalFailureAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{claim.ErrInvalidEvidence, 400}, {filedomain.ErrClaimEvidenceImageNotAvailable, 400}, {errors.New("database read secret"), 500}} {
		s := newServiceMock(t)
		s.On("Submit", mock.Anything, "subject", mock.Anything, mock.Anything).Return((*claim.SubmissionResult)(nil), tc.err).Once()
		w := perform(s, "POST", "/claims", `{"reference":{"job_request_id":1},"reason":"damage","description":"x"}`, true)
		require.Equal(t, tc.status, w.Code)
		require.NotContains(t, w.Body.String(), "database read secret")
	}
}

func TestDetailRejectsInvalidClaimIDBeforeService(t *testing.T) {
	for _, id := range []string{"2147483648", "999999999999999999999999", "-1", "+1", "0", "01", "abc"} {
		s := newServiceMock(t)
		response := perform(s, "GET", "/claims/"+id, "", true)
		require.Equal(t, 404, response.Code)
		require.Empty(t, s.Calls)
	}
}
