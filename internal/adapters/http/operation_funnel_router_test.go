package httpadapter

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_funnel_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func funnelMetricsRoute(t *testing.T, service *operationFunnelQueryServiceMock) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := operation_funnel_handler.NewHandler(service)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), OperationFunnelHandler: handler})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.registerAdminRoutes(engine, authentication)
	return engine
}

func TestAdminFunnelRouteRequiresExactPermissionAndSetsPrivateCache(t *testing.T) {
	service := &operationFunnelQueryServiceMock{}
	observedAt := time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)
	service.On("Query", mock.Anything, operation.FunnelQuery{}).Return(readmodel.FunnelMetrics{From: observedAt.Add(-30 * 24 * time.Hour), To: observedAt, ObservedAt: observedAt, TimeZone: "America/Argentina/Buenos_Aires", Rounding: "half_up", DecimalPlaces: 2}, nil).Once()
	engine := funnelMetricsRoute(t, service)
	adminRoleToken, err := buildTokenWithRole("auth0|admin-role", "admin")
	require.NoError(t, err)

	for _, test := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "other administrative permission", token: auth0.NewTokenBuilder().BuildToken("auth0|operator", []string{"read:admin_operations"}), wantStatus: http.StatusForbidden},
		{name: "role without metric permission", token: adminRoleToken, wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/admin/metrics/funnel", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.wantStatus, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		})
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/metrics/funnel", nil)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|operator", []string{"read:admin_metrics"}))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Body.String(), `"observed_at":"2026-09-30T15:00:00Z"`)
	service.AssertExpectations(t)
}

func TestAdminFunnelRouteRejectsMalformedQueryBeforeReader(t *testing.T) {
	service := &operationFunnelQueryServiceMock{}
	engine := funnelMetricsRoute(t, service)
	for _, path := range []string{
		"/admin/metrics/funnel?from=2026-09-01T00:00:00Z",
		"/admin/metrics/funnel?category_id=0",
		"/admin/metrics/funnel?diagnostico=true",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|operator", []string{"read:admin_metrics"}))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusBadRequest, response.Code, path)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"), path)
		require.NotContains(t, response.Body.String(), "cohorts", path)
	}
	service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything)
}

func TestAdminFunnelRouteMapsDomainRangeValidationToBadRequest(t *testing.T) {
	service := &operationFunnelQueryServiceMock{}
	service.On("Query", mock.Anything, mock.MatchedBy(func(query operation.FunnelQuery) bool {
		return query.Period != nil && query.Period.From.After(query.Period.To)
	})).Return(readmodel.FunnelMetrics{}, operation.ErrInvalidFunnelQuery).Once()
	engine := funnelMetricsRoute(t, service)
	request := httptest.NewRequest(http.MethodGet, "/admin/metrics/funnel?from=2026-09-03T00:00:00Z&to=2026-09-02T00:00:00Z", nil)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|operator", []string{"read:admin_metrics"}))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"error":"invalid funnel query"}`, response.Body.String())
	require.NotContains(t, response.Body.String(), "cohorts")
	service.AssertExpectations(t)
}

func TestAdminFunnelRouteDoesNotReturnEmptyMetricsOnReaderFailure(t *testing.T) {
	service := &operationFunnelQueryServiceMock{}
	service.On("Query", mock.Anything, operation.FunnelQuery{}).Return(readmodel.FunnelMetrics{}, errors.New("snapshot read failed")).Once()
	engine := funnelMetricsRoute(t, service)
	request := httptest.NewRequest(http.MethodGet, "/admin/metrics/funnel", nil)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|operator", []string{"read:admin_metrics"}))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
	require.NotContains(t, response.Body.String(), "snapshot read failed")
	require.NotContains(t, response.Body.String(), "cohorts")
	service.AssertExpectations(t)
}
