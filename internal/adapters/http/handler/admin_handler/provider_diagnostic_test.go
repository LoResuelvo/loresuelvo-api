package admin_handler

import (
	"encoding/json"
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
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
	require.Len(t, body, 8)
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

func TestDiagnosticMapperKeepsTypedReferencesAndPermissionSeparated(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	d := &readmodel.ProviderDiagnostic{Provider: readmodel.Provider{ID: 12}, Activity: []readmodel.ProviderActivity{{Type: "work_order", ID: 9, Status: "paid", OccurredOn: now}}, Reviews: []readmodel.ProviderReview{{WorkOrderID: 9, Rating: 5, Description: "Persisted"}}, OrderSync: []readmodel.ProviderOrderSync{{WorkOrderID: 9}}, RatingStats: provider.RatingStats{Count: 8, Total: 39}}
	response := diagnosticResponseFromModel(d)
	require.Equal(t, "/admin/operations?provider_id=12", response.Navigation.OperationsURL)
	require.Equal(t, "read:admin_operations", response.Navigation.RequiredPermission)
	require.Equal(t, 8, response.Reputation.Count)
	require.Equal(t, 4.9, response.Reputation.Average)
	require.Len(t, response.Activity, 1)
	require.Equal(t, 9, response.Activity[0].ID)
	require.Equal(t, "work_order", response.Activity[0].Type)
	require.Len(t, response.Calendar.OrderSync, 1)
	require.Nil(t, response.Calendar.OrderSync[0].SyncedOn)
}
