package middleware

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInstallationLoggingSuppressesPrivateRequestResponseQueryAndPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, method := range []string{"PUT", "DELETE"} {
		for _, status := range []int{200, 400, 401, 403, 409, 500} {
			var logs bytes.Buffer
			engine := gin.New()
			engine.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
			engine.Handle(method, "/installations/:installation_id", func(c *gin.Context) { c.JSON(status, gin.H{"fcm_token": "private-response-token"}) })
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(method, "/installations/private-path-secret?category_id=private-query-secret", strings.NewReader(`{"installation_secret":"private-body-secret"}`)))
			require.Equal(t, status, response.Code)
			require.Contains(t, logs.String(), "http.request.completed")
			for _, secret := range []string{"private-path-secret", "private-query-secret", "private-body-secret", "private-response-token", "http.path_params", "http.request_body", "http.response_body", "http.query_params"} {
				require.NotContains(t, logs.String(), secret)
			}
		}
	}
}
