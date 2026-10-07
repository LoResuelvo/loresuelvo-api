package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDefaultAuthenticationErrorContractIsUnchanged(t *testing.T) {
	layer, err := BaseAutheticationLayer(auth0.NewFakeValidator())
	require.NoError(t, err)
	engine := gin.New()
	engine.GET("/", layer, func(c *gin.Context) { t.Fatal("invalid authentication reached handler") })
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	require.Equal(t, 401, response.Code)
	require.JSONEq(t, `{"error":"invalid_token","message":"Failed to validate JWT."}`, response.Body.String())
}
func TestDefaultAuthenticationErrorAndValidSubject(t *testing.T) {
	layer, err := BaseAutheticationLayer(auth0.NewFakeValidator())
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Header("Cache-Control", "private, no-store"); c.Next() })
	engine.GET("/claims", layer, func(c *gin.Context) {
		id, ok := GetUserID(c)
		require.True(t, ok)
		require.Equal(t, "auth0|claim-subject", id)
		c.Status(200)
	})
	for _, token := range []string{"", "invalid", auth0.NewTokenBuilder().BuildToken("auth0|claim-subject", nil)} {
		request := httptest.NewRequest("GET", "/claims", nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		if token == "" || token == "invalid" {
			require.Equal(t, 401, response.Code)
			require.JSONEq(t, `{"error":"invalid_token","message":"Failed to validate JWT."}`, response.Body.String())
		} else {
			require.Equal(t, 200, response.Code)
		}
	}
}
