package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORSLayerAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORSLayer(CORSConfig{
		AllowedOrigins: []string{"http://localhost:8081"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "http://localhost:8081", resp.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", resp.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Authorization, Content-Type", resp.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORSLayerDoesNotAllowUnknownOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORSLayer(CORSConfig{
		AllowedOrigins: []string{"http://localhost:8081"},
		AllowedMethods: []string{http.MethodGet, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://malicious.example")
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSLayerAnswersPreflightRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORSLayer(CORSConfig{
		AllowedOrigins: []string{"http://localhost:8081"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))

	req := httptest.NewRequest(http.MethodOptions, "/categories", nil)
	req.Header.Set("Origin", "http://localhost:8081")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
	assert.Equal(t, "http://localhost:8081", resp.Header().Get("Access-Control-Allow-Origin"))
}
