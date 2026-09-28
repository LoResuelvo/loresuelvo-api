package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRequestLoggerSuppressesPrivateConsumerHistoryAndProviderDiagnostic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []string{"/admin/consumers/:id/history", "/admin/providers/:id/diagnostic"} {
		for _, status := range []int{200, 400, 401, 403, 404, 500} {
			t.Run(route+"/"+strconv.Itoa(status), func(t *testing.T) {
				var logs bytes.Buffer
				r := gin.New()
				r.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
				r.GET(route, func(c *gin.Context) {
					body, readErr := io.ReadAll(c.Request.Body)
					require.NoError(t, readErr)
					require.Contains(t, string(body), "private-request-body")
					c.JSON(status, gin.H{"address": "private-home-location", "profile_photo_url": "private-url-sentinel", "error": "private-error-detail"})
				})
				path := strings.Replace(route, ":id", "12", 1)
				req := httptest.NewRequest("GET", path+"?category_id=private-query-sentinel&cursor=private-cursor-sentinel", strings.NewReader(`{"payload":"private-request-body"}`))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				require.Equal(t, status, rec.Code)
				require.Contains(t, rec.Body.String(), "private-home-location")
				require.Contains(t, logs.String(), "http.request.completed")
				require.Contains(t, logs.String(), "http.status_code")
				for _, secret := range []string{"private-home-location", "private-url-sentinel", "private-error-detail", "private-query-sentinel", "private-cursor-sentinel", "private-request-body", "http.response_body", "http.request_body", "http.query_params"} {
					require.NotContains(t, logs.String(), secret)
				}
			})
		}
	}
}
func TestPrivateAdminProfileReadClassifierDoesNotBroadenOrdinaryRoutes(t *testing.T) {
	for _, path := range []string{"/admin/consumers/12/history", "/admin/consumers/invalid/history", "/admin/providers/12/diagnostic"} {
		require.True(t, isPrivateAdminReadPath(path))
	}
	for _, path := range []string{"/admin/consumers", "/admin/providers", "/admin/consumers/history", "/admin/providers/diagnostic", "/admin/consumers/12/not-history", "/ordinary/history"} {
		require.False(t, isPrivateAdminReadPath(path))
	}
}
