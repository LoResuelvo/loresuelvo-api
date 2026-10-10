package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProviderSearchRouteRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ProviderHandler: provider_handler.NewProviderHandler(nil)})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	for _, test := range []struct {
		name, token string
		status      int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"invalid", "invalid", http.StatusUnauthorized},
		{"authenticated without additional permission", auth0.NewTokenBuilder().BuildToken("auth0|consumer", nil), http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/providers", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code)
		})
	}
}
