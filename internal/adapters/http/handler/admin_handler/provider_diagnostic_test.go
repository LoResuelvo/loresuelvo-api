package admin_handler

import (
	"encoding/json"
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func diagnosticRouter(service *diagnosticServiceMock) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	r.GET("/admin/providers/:provider_id/diagnostic", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth0|support") }, NewProviderDiagnosticHandler(service).Get)
	return r
}
func TestDiagnosticHandlerMapsSafeFieldsAndNullEvidence(t *testing.T) {
	s := new(diagnosticServiceMock)
	d := &readmodel.ProviderDiagnostic{Provider: readmodel.Provider{ID: 12, ProfilePhotoFileID: "internal-photo", ProfilePhotoURL: "https://cdn.example/photo"}}
	d.DiagnosticChecks = d.Checks(time.Now())
	s.On("Query", mock.Anything, "12", "auth0|support", "diagnostic-test").Return(d, nil).Once()
	req := httptest.NewRequest(http.MethodGet, "/admin/providers/12/diagnostic", nil)
	req.Header.Set("X-Request-ID", "diagnostic-test")
	rec := httptest.NewRecorder()
	diagnosticRouter(s).ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 4)
	require.NotContains(t, rec.Body.String(), "internal-photo")
	require.NotContains(t, rec.Body.String(), "IdentityResultOn")
	require.Contains(t, rec.Body.String(), `"evidence_on":null`)
	require.Contains(t, rec.Body.String(), `"coverage_zones":[]`)
	s.AssertExpectations(t)
}
func TestDiagnosticHandlerDoesNotLeakDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		body string
	}{{admin.ErrInvalidDiagnosticProviderID, 400, "invalid provider ID"}, {admin.ErrProviderDiagnosticNotFound, 404, "provider not found"}, {errors.New("private storage details"), 500, "internal server error"}} {
		s := new(diagnosticServiceMock)
		s.On("Query", mock.Anything, "12", "auth0|support", "diagnostic-test").Return(nil, tc.err)
		req := httptest.NewRequest(http.MethodGet, "/admin/providers/12/diagnostic", nil)
		req.Header.Set("X-Request-ID", "diagnostic-test")
		rec := httptest.NewRecorder()
		diagnosticRouter(s).ServeHTTP(rec, req)
		require.Equal(t, tc.code, rec.Code)
		require.JSONEq(t, `{"error":"`+tc.body+`"}`, rec.Body.String())
	}
}
