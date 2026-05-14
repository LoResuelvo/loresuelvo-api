package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNewAuth0MiddlewareFromEnvRequiresDomain(t *testing.T) {
	t.Setenv("AUTH0_DOMAIN", "")
	t.Setenv("AUTH0_AUDIENCE", "https://api.example.test")

	middleware, err := NewAuth0MiddlewareFromEnv()

	require.Nil(t, middleware)
	require.EqualError(t, err, "AUTH0_DOMAIN is required")
}

func TestNewAuth0MiddlewareFromEnvRequiresAudience(t *testing.T) {
	t.Setenv("AUTH0_DOMAIN", "tenant.example.test")
	t.Setenv("AUTH0_AUDIENCE", "")

	middleware, err := NewAuth0MiddlewareFromEnv()

	require.Nil(t, middleware)
	require.EqualError(t, err, "AUTH0_AUDIENCE is required")
}

func TestAuthenticatedUserRequiresValidatedClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/me", AuthenticatedUser)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"message":"Failed to get token claims."}`, response.Body.String())
}
