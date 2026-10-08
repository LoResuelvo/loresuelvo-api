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

func TestAdminReviewRequestLoggerPreservesTechnicalLogsWithoutRestrictedContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []struct{ method, path string }{{"GET", "/admin/reviews"}, {"GET", "/admin/reviews/1"}, {"POST", "/admin/reviews/1/moderate"}} {
		for _, status := range []int{200, 400, 401, 403, 404, 409, 500} {
			var logs bytes.Buffer
			engine := gin.New()
			engine.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
			engine.Handle(route.method, route.path, func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, "private-reason", string(body))
				observability.LoggerFromContextOr(c.Request.Context(), slog.Default()).InfoContext(c.Request.Context(), "security.access.checked", "operation", "review_moderation")
				c.JSON(status, gin.H{"description": "private-original", "explanation": "private-explanation"})
			})
			request := httptest.NewRequest(route.method, route.path+"?reason=private-query", strings.NewReader("private-reason"))
			request.Header.Set("Authorization", "Bearer private-token")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, status, response.Code)
			require.Contains(t, logs.String(), "http.request.completed")
			require.Contains(t, logs.String(), "security.access.checked")
			require.Contains(t, logs.String(), "request_id")
			require.Contains(t, logs.String(), "http.status_code")
			for _, secret := range []string{"private-reason", "private-original", "private-explanation", "private-query", "private-token", "http.request_body", "http.response_body", "http.query_params"} {
				require.NotContains(t, logs.String(), secret)
			}
		}
	}
}
