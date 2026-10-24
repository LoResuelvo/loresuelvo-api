package claim_handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	claimmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdministrativeResolutionRejectsMalformedOrSpoofedInput(t *testing.T) {
	for _, body := range []string{
		`{"type":"agreement","reasoning":"valid","actor_id":1}`,
		`{"type":"agreement","reasoning":"valid","resolved_on":"2026-01-01"}`,
		`{"type":"agreement","reasoning":"valid","reasoning":"different"}`,
		`{"type":"agreement","reasoning":"valid","suggested_compensation":{"amount_minor":1,"currency":"ARS"}}`,
		`{"type":"agreement","reasoning":"valid","suggested_compensation":{"amount_minor":1,"unit":"minor"}}`,
		`{"type":"agreement","reasoning":"valid","suggested_compensation":{"amount_minor":1.5}}`,
		`{"type":"agreement","reasoning":"valid","suggested_compensation":{"amount_minor":9223372036854775808}}`,
		`{"type":"agreement","reasoning":"valid","suggested_compensation":{"amount_minor":null}}`,
		`{"type":"agreement","reasoning":"valid","suggested_compensation":null}`,
		`{"type":"agreement","reasoning":"valid"} {}`,
		"{\"type\":\"agreement\",\"reasoning\":\"\xff\"}",
		`{"type":"agreement","reasoning":"valid\u0000invalid"}`,
	} {
		t.Run(fmt.Sprintf("%q", body), func(t *testing.T) {
			service := new(adminServiceMock)
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
			router.POST("/admin/claims/:id/resolution", NewAdminHandler(service).Resolve)
			req := httptest.NewRequest("POST", "/admin/claims/1/resolution", strings.NewReader(body))
			req.Header.Set("Idempotency-Key", uuid.NewString())
			result := httptest.NewRecorder()
			router.ServeHTTP(result, req)
			require.Equal(t, 400, result.Code, result.Body.String())
			service.AssertNotCalled(t, "Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestAdministrativeResolutionDerivesConstantsAndUsesNormalizedInput(t *testing.T) {
	service := new(adminServiceMock)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	key := uuid.NewString()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
	router.POST("/admin/claims/:id/resolution", NewAdminHandler(service).Resolve)
	service.On("Resolve", mock.Anything, "admin", 1, key, "test-correlation", mock.MatchedBy(func(input claim.ResolutionInput) bool {
		return input.Reasoning == "valid" && input.SuggestedAmountMinor != nil && *input.SuggestedAmountMinor == 1500
	})).Return(&claim.AdministrationResult{Claim: &claim.Claim{ID: 1, Status: claim.StatusResolved}}, nil).Once()
	request := httptest.NewRequest("POST", "/admin/claims/1/resolution", strings.NewReader(`{"type":"agreement","reasoning":"  valid  ","suggested_compensation":{"amount_minor":1500}}`))
	request.Header.Set("Idempotency-Key", key)
	request.Header.Set("X-Request-ID", "test-correlation")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code, response.Body.String())
	service.AssertExpectations(t)
}

func TestAdministrativeDetailUnavailableLinkedEvidenceRemainsServerError(t *testing.T) {
	service := new(adminServiceMock)
	cause := fmt.Errorf("%w: %w", claim.ErrEvidenceAccessUnavailable, filedomain.ErrClaimEvidenceImageNotAvailable)
	service.On("Get", mock.Anything, "admin", 1, "evidence-correlation").Return((*claim.AdminDetail)(nil), cause).Once()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
	router.GET("/admin/claims/:id", NewAdminHandler(service).Get)
	req := httptest.NewRequest("GET", "/admin/claims/1", nil)
	req.Header.Set("X-Request-ID", "evidence-correlation")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	require.Equal(t, 500, response.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestAdministrativeClaimListIncludesClaimantName(t *testing.T) {
	service := new(adminServiceMock)
	service.On("List", mock.Anything, mock.Anything).Return(&claim.AdminPage{Claims: []claimmodel.AdminClaimSummary{{
		ClaimSummary:  claimmodel.ClaimSummary{ID: 1},
		ClaimantID:    12,
		ClaimantParty: "consumer",
		ClaimantName:  "Ana Perez",
		ClaimantEmail: "ana@example.com",
	}}}, nil).Once()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
	router.GET("/admin/claims", NewAdminHandler(service).List)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/admin/claims", nil))

	require.Equal(t, 200, response.Code, response.Body.String())
	var body struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	require.JSONEq(t, `"Ana Perez"`, string(body.Items[0]["claimant_name"]))
	service.AssertExpectations(t)
}

func TestAdministrativeClaimDetailIncludesClaimantAndRespondentIdentity(t *testing.T) {
	service := new(adminServiceMock)
	service.On("Get", mock.Anything, "admin", 1, "claim-identity-correlation").Return(&claim.AdminDetail{
		GetResult:     claim.GetResult{Claim: &claim.Claim{ID: 1, ClaimantID: 12, ClaimantParty: claim.PartyConsumer}},
		ClaimantName:  "Ana Perez",
		ClaimantEmail: "ana@example.com",
		Respondent:    claimmodel.AdminIdentity{ID: 34, Party: "provider", Name: "Juan Gomez", Email: "juan@example.com"},
	}, nil).Once()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
	router.GET("/admin/claims/:id", NewAdminHandler(service).Get)
	request := httptest.NewRequest("GET", "/admin/claims/1", nil)
	request.Header.Set("X-Request-ID", "claim-identity-correlation")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, 200, response.Code, response.Body.String())
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.JSONEq(t, `{"id":12,"party":"consumer","name":"Ana Perez","email":"ana@example.com"}`, string(body["claimant"]))
	require.JSONEq(t, `{"id":34,"party":"provider","name":"Juan Gomez","email":"juan@example.com"}`, string(body["respondent"]))
	service.AssertExpectations(t)
}
