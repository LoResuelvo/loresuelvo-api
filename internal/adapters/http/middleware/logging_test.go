package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggerDoesNotLogIdentityVerificationCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	engine := gin.New()
	engine.Use(RequestLogger(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))))
	engine.POST("/providers/me/identity-verification-sessions", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"session_token": "temporary-secret", "verification_url": "https://verify.example/private"})
	})
	request := httptest.NewRequest(http.MethodPost, "/providers/me/identity-verification-sessions", nil)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, output.String(), "temporary-secret")
	require.NotContains(t, output.String(), "verify.example")
	require.True(t, strings.Contains(output.String(), "http.request.completed"))
}
