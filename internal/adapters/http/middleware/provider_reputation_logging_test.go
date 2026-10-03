package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggerSuppressesProviderReputationContentsAndPreservesTechnicalLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []int{200, 400, 401, 403, 404, 500} {
		var logs bytes.Buffer
		router := gin.New()
		router.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
		router.GET("/providers/me/statistics/reputation", func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.Equal(t, "private-request", string(body))
			observability.LoggerFromContextOr(c.Request.Context(), slog.Default()).InfoContext(c.Request.Context(), "security.access.checked", "operation", "read_reputation")
			c.JSON(status, gin.H{"description": "private-review"})
		})
		request := httptest.NewRequest("GET", "/providers/me/statistics/reputation?category_id=private-query&cursor=private-cursor", strings.NewReader("private-request"))
		request.Header.Set("Authorization", "Bearer private-token")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, status, response.Code)
		require.Contains(t, logs.String(), "http.request.completed")
		require.Contains(t, logs.String(), "security.access.checked")
		require.Contains(t, logs.String(), "request_id")
		require.Contains(t, logs.String(), "http.status_code")
		for _, secret := range []string{"private-request", "private-review", "private-query", "private-cursor", "private-token", "http.request_body", "http.response_body", "http.query_params"} {
			require.NotContains(t, logs.String(), secret)
		}
	}
}

func TestProviderReputationPrivacyClassifierIsExact(t *testing.T) {
	require.True(t, isPrivateProviderActivityPath("/providers/me/statistics/reputation"))
	for _, path := range []string{"/providers/me/statistics/reputation/extra", "/providers/1/statistics/reputation", "/providers/me/statistics/reputation-public"} {
		require.False(t, isPrivateProviderActivityPath(path), path)
	}
}
