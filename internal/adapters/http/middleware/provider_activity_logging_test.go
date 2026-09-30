package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggerSuppressesProviderActivityContents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []int{200, 400, 401, 403, 404, 500} {
		var logs bytes.Buffer
		router := gin.New()
		router.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
		router.GET("/providers/me/statistics/activity", func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), "private-request")
			c.JSON(status, gin.H{"value": "private-response"})
		})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/providers/me/statistics/activity?category_id=private-query", strings.NewReader("private-request")))
		require.Equal(t, status, response.Code)
		require.Contains(t, logs.String(), "http.status_code")
		for _, secret := range []string{"private-request", "private-response", "private-query", "http.request_body", "http.response_body", "http.query_params"} {
			require.NotContains(t, logs.String(), secret)
		}
	}
}

func TestProviderActivityPrivacyClassifierIsExact(t *testing.T) {
	require.True(t, isPrivateProviderActivityPath("/providers/me/statistics/activity"))
	for _, path := range []string{"/providers/me/statistics", "/providers/me/statistics/activity/extra", "/providers/1/statistics/activity"} {
		require.False(t, isPrivateProviderActivityPath(path))
	}
}
