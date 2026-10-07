package httpadapter

import (
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/claim_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaimRoutesSetPrivateCacheBeforeAuthentication(t *testing.T) {
	svc := new(claimServiceMock)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ClaimHandler: claim_handler.NewHandler(svc)})
	auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router.registerClaimRoutes(engine, auth)
	for _, route := range []struct{ method, path string }{{"POST", "/claims"}, {"GET", "/claims"}, {"GET", "/claims/1"}, {"GET", "/claims/1/images/file"}} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		require.Equal(t, 401, response.Code)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		require.JSONEq(t, `{"error":"invalid_token","message":"Failed to validate JWT."}`, response.Body.String())
	}
	require.Empty(t, svc.Calls)
}

func TestClaimRoutesKeepCachePolicyOnServiceResults(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		status             int
		err                error
	}{
		{"invalid submission", "POST", "/claims", 400, nil},
		{"forbidden account", "GET", "/claims/1", 403, claim.ErrForbidden},
		{"missing resource", "GET", "/claims/1", 404, claim.ErrNotFound},
		{"conflicting key", "POST", "/claims", 409, claim.ErrSubmissionKeyConflict},
		{"technical failure", "GET", "/claims/1", 500, errors.New("private storage failure")},
		{"detail", "GET", "/claims/1", 200, nil},
		{"created", "POST", "/claims", 201, nil},
		{"image URL", "GET", "/claims/1/images/file", 200, nil},
		{"empty list", "GET", "/claims", 200, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := new(claimServiceMock)
			body := `{"reference":{"job_request_id":1},"reason":"damage","description":"x"}`
			switch {
			case tc.status == 400:
				body = `{}`
			case tc.path == "/claims" && tc.method == "GET":
				svc.On("List", mock.Anything, "subject", mock.Anything).Return(&claim.Page{}, nil).Once()
			case strings.Contains(tc.path, "/images/"):
				svc.On("ResolveImage", mock.Anything, "subject", 1, "file").Return("https://private.example/image?signature=temporary", nil).Once()
			case tc.method == "POST":
				var result *claim.SubmissionResult
				if tc.err == nil {
					result = &claim.SubmissionResult{Claim: &claim.Claim{ID: 1}, Created: true}
				}
				svc.On("Submit", mock.Anything, "subject", mock.Anything, mock.Anything).Return(result, tc.err).Once()
			default:
				var found *claim.Claim
				if tc.err == nil {
					found = &claim.Claim{ID: 1}
				}
				svc.On("Get", mock.Anything, "subject", 1).Return(found, tc.err).Once()
			}
			router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ClaimHandler: claim_handler.NewHandler(svc)})
			auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
			require.NoError(t, err)
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			router.registerClaimRoutes(engine, auth)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("subject", nil))
			req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			require.NotContains(t, response.Body.String(), "private storage failure")
			for _, header := range []string{"X-Storage-Key", "X-Storage-Bucket", "Authorization"} {
				require.Empty(t, response.Header().Get(header))
			}
			svc.AssertExpectations(t)
		})
	}
}
