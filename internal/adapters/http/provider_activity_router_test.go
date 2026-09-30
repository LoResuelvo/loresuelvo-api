package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProviderActivityRouteRequiresAuthenticationAndPreventsCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ActivityHandler: provider_handler.NewActivityHandler(nil)})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	for _, test := range []struct {
		name, token string
		status      int
	}{
		{"missing", "", 401},
		{"invalid", "invalid", 401},
		{"authenticated without admin permission", auth0.NewTokenBuilder().BuildToken("auth0|provider", nil), 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/providers/me/statistics/activity?provider_id=2", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		})
	}
}

func TestProviderActivityRouteRejectsMalformedQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ActivityHandler: provider_handler.NewActivityHandler(nil)})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)

	for _, test := range []struct {
		name  string
		query string
	}{
		{"malformed escape", "from=%GG"},
		{"duplicate key", "compare_previous=true&compare_previous=false"},
		{"unknown provider selector", "provider_id=2"},
		{"timestamp without zone", "from=2026-09-01T00%3A00%3A00"},
		{"invalid comparison", "compare_previous=TRUE"},
		{"empty value", "granularity="},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/providers/me/statistics/activity?"+test.query, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			require.JSONEq(t, `{"error":"invalid activity query"}`, response.Body.String())
		})
	}
}

func TestProviderActivityRouteRejectsInvalidGranularity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := provider.NewActivityService(nil, nil, clockadapter.NewSystemClock())
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ActivityHandler: provider_handler.NewActivityHandler(service)})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	request := httptest.NewRequest(http.MethodGet, "/providers/me/statistics/activity?granularity=quarter", nil)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"error":"invalid activity query"}`, response.Body.String())
}
