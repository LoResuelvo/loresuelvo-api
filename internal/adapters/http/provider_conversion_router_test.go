package httpadapter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProviderConversionRouteRequiresJWTAndPreventsCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	for _, test := range []struct {
		name, token string
		status      int
	}{
		{"missing", "", 401}, {"invalid", "invalid", 401}, {"own JWT without admin permissions", token, 200}, {"bad range", token, 400}, {"consumer", token, 403}, {"missing profile", token, 404}, {"read error", token, 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &conversionServiceMock{}
			if test.status != 401 {
				var result *readmodel.Conversion
				var queryErr error
				switch test.status {
				case 400:
					queryErr = provider.ErrInvalidConversionQuery
				case 403:
					queryErr = provider.ErrConversionForbidden
				case 404:
					queryErr = provider.ErrConversionProviderNotFound
				case 500:
					queryErr = errors.New("internal")
				default:
					result = &readmodel.Conversion{}
				}
				service.On("Query", mock.Anything, "auth0|provider", mock.Anything).Return(result, queryErr).Once()
			}
			router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ConversionHandler: provider_handler.NewConversionHandler(service)})
			authentication, err := router.middlewareSetup()
			require.NoError(t, err)
			engine := gin.New()
			router.registerProviderRoutes(engine, authentication)
			request := httptest.NewRequest(http.MethodGet, "/providers/me/statistics/conversion", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			service.AssertExpectations(t)
			if test.status == 401 {
				service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}
func TestProviderConversionRouteRejectsUnexpectedQueryOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &conversionServiceMock{}
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ConversionHandler: provider_handler.NewConversionHandler(service)})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	for _, query := range []string{"provider_id=1", "cursor=abc", "granularity=day", "compare_previous=true", "as_of=2026-09-01T00:00:00Z", "from=%GG", "from=", "to=a&to=b"} {
		request := httptest.NewRequest(http.MethodGet, "/providers/me/statistics/conversion?"+query, nil)
		request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code, query)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
	}
}
