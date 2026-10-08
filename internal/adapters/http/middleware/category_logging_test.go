package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCategoryAdministrationLoggerPreservesMetadataWithoutPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []struct{ method, path, query string }{
		{"PATCH", "/categories/1", "reason=private-query"},
		{"GET", "/admin/categories/1/impact", "context=private-query"},
		{"GET", "/categories", "include_disabled=true&context=private-query"},
	} {
		for _, status := range []int{200, 400, 401, 403, 404, 409, 500} {
			t.Run(route.method+route.path+strconv.Itoa(status), func(t *testing.T) {
				var logs bytes.Buffer
				engine := gin.New()
				engine.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
				engine.Handle(route.method, route.path, func(c *gin.Context) {
					body, err := io.ReadAll(c.Request.Body)
					require.NoError(t, err)
					require.Contains(t, string(body), "private-reason")
					c.JSON(status, gin.H{"name": "private-response"})
				})
				request := httptest.NewRequest(route.method, route.path+"?"+route.query, strings.NewReader(`{"reason":"private-reason"}`))
				request.Header.Set("Authorization", "Bearer private-token")
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				require.Equal(t, status, response.Code)
				require.Contains(t, logs.String(), "http.request.completed")
				require.Contains(t, logs.String(), "http.status_code")
				require.Contains(t, logs.String(), "request_id")
				for _, secret := range []string{"private-reason", "private-response", "private-query", "private-token", "http.request_body", "http.response_body", "http.query_params"} {
					require.NotContains(t, logs.String(), secret)
				}
			})
		}
	}
}
