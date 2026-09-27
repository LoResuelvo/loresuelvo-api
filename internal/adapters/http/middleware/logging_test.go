package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggerExposesValidatedRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		requestID  string
		wantHeader string
	}{
		{name: "preserves valid request ID", requestID: "category-create:abc-123", wantHeader: "category-create:abc-123"},
		{name: "replaces invalid request ID", requestID: "bad\nid", wantHeader: "generated"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
			var observedRequestID string
			var found bool
			router.GET("/", func(c *gin.Context) {
				observedRequestID, found = GetRequestID(c)
				c.Status(204)
			})

			request := httptest.NewRequest("GET", "/", nil)
			if test.requestID != "" {
				request.Header.Set(requestIDHeader, test.requestID)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, 204, recorder.Code)
			require.True(t, found)
			require.NotEmpty(t, observedRequestID)
			if test.wantHeader == "generated" {
				require.NotEqual(t, test.requestID, observedRequestID)
				require.Equal(t, observedRequestID, recorder.Header().Get(requestIDHeader))
			} else {
				require.Equal(t, test.wantHeader, observedRequestID)
				require.Equal(t, test.wantHeader, recorder.Header().Get(requestIDHeader))
			}
		})
	}
}

func TestRequestLoggerDoesNotCaptureAuditLogResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	router.GET("/admin/audit-logs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reason": "private reconciliation reason"})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs?category_id=private-reconciliation-reason", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "private reconciliation reason")
	require.Contains(t, logs.String(), "http.request.completed")
	require.NotContains(t, logs.String(), "private reconciliation reason")
	require.NotContains(t, logs.String(), "http.response")
	require.NotContains(t, logs.String(), "private-reconciliation-reason")
	require.NotContains(t, logs.String(), "http.query_params")
}

func TestRequestLoggerDoesNotCaptureOperationDetailResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name   string
		route  string
		path   string
		status int
	}{
		{name: "existing operation", route: "/admin/operations/:operation_id", path: "/admin/operations/jr-123", status: http.StatusOK},
		{name: "invalid operation ID", route: "/admin/operations/:operation_id", path: "/admin/operations/invalid", status: http.StatusBadRequest},
		{name: "missing operation", route: "/admin/operations/:operation_id", path: "/admin/operations/sp-999", status: http.StatusNotFound},
		{name: "nested private media", route: "/admin/operations/:operation_id/media/:file_id", path: "/admin/operations/jr-123/media/file-456", status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			router := gin.New()
			router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
			router.GET(test.route, func(c *gin.Context) {
				c.JSON(test.status, gin.H{"reason": "private operator reason"})
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path+"?data.id=private-provider-reference", nil))

			require.Equal(t, test.status, response.Code)
			require.Contains(t, response.Body.String(), "private operator reason")
			require.Contains(t, logs.String(), "http.request.completed")
			require.Contains(t, logs.String(), "http.route="+test.route)
			require.Contains(t, logs.String(), "http.path_params")
			require.NotContains(t, logs.String(), "private operator reason")
			require.NotContains(t, logs.String(), "private-provider-reference")
			require.NotContains(t, logs.String(), "http.response")
			require.NotContains(t, logs.String(), "http.query_params")
		})
	}
}

func TestRequestLoggerStillCapturesOperationInboxResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	router.GET("/admin/operations", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"summary": "inbox summary"})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations?category_id=42", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, logs.String(), "inbox summary")
	require.Contains(t, logs.String(), "http.response")
	require.Contains(t, logs.String(), "http.query_params")
}
