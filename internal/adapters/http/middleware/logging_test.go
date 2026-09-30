package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRequestLoggerDoesNotCaptureAdministrativePaymentPayloadOrCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	router.GET("/admin/payments", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"external_payment_id": "private-payment-id", "amount_cents": 9001})
	})
	request := httptest.NewRequest(http.MethodGet, "/admin/payments?cursor=private-signed-cursor&external_payment_id=private-payment-id", strings.NewReader(`{"payload":"private-payment-body"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "private-payment-id")
	require.Contains(t, logs.String(), "http.request.completed")
	for _, private := range []string{"private-payment-id", "private-signed-cursor", "private-payment-body", "http.request_body", "http.response_body", "http.query_params"} {
		require.NotContains(t, logs.String(), private)
	}
}

func TestRequestLoggerDoesNotCaptureFunnelAggregatesOrCategoryFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/admin/metrics/funnel", "/admin/metrics/funnel/"} {
		t.Run(path, func(t *testing.T) {
			var logs bytes.Buffer
			router := gin.New()
			router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
			router.GET(path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"conversion_percentage": 66.67, "category_source": "provider.current_category_id"})
			})
			request := httptest.NewRequest(http.MethodGet, path+"?category_id=private-category-id", strings.NewReader(`{"filter":"private-funnel-filter"}`))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code)
			require.Contains(t, response.Body.String(), "provider.current_category_id")
			require.Contains(t, logs.String(), "http.request.completed")
			for _, private := range []string{"private-category-id", "private-funnel-filter", "provider.current_category_id", "conversion_percentage", "http.request_body", "http.response_body", "http.query_params"} {
				require.NotContains(t, logs.String(), private)
			}
		})
	}
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

func TestRequestLoggerDoesNotCapturePrivateOperationChatOrAuditReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	router.GET("/admin/operations/:operation_id/conversation", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"messages": []gin.H{{"content": "private-chat-content", "images": []gin.H{{"url": "https://storage.invalid/private-key?secret-signature"}}}}})
	})
	request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-123/conversation?cursor=private-cursor", strings.NewReader(`{"payload":"private-get-body"}`))
	request.Header.Set("X-Audit-Reason", "private-audit-reason")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.Contains(t, logs.String(), "http.request.completed")
	for _, private := range []string{"private-chat-content", "private-key", "secret-signature", "private-audit-reason", "private-get-body", "private-cursor", "http.request_body", "http.response_body", "http.query_params"} {
		require.NotContains(t, logs.String(), private)
	}
}

func TestRequestLoggerDoesNotCaptureParticipantWorkChatPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct{ method, path, route string }{{"GET", "/conversations", "/conversations"}, {"GET", "/conversations/12", "/conversations/:conversationID"}, {"POST", "/conversations/12/messages", "/conversations/:conversationID/messages"}} {
		t.Run(test.method+test.path, func(t *testing.T) {
			var logs bytes.Buffer
			router := gin.New()
			router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
			router.Handle(test.method, test.route, func(c *gin.Context) {
				c.JSON(200, gin.H{"content": "private-response-chat", "url": "https://storage.invalid/private-signed-url?secret"})
			})
			request := httptest.NewRequest(test.method, test.path+"?cursor=private-cursor", strings.NewReader(`{"content":"private-request-chat"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer private-token")
			request.Header.Set("X-Request-ID", "private-work-chat-test")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, 200, response.Code)
			require.Contains(t, logs.String(), "http.request.completed")
			require.Contains(t, logs.String(), "http.route="+test.route)
			require.Contains(t, logs.String(), "http.method="+test.method)
			require.Contains(t, logs.String(), "http.status_code=200")
			require.Contains(t, logs.String(), "request_id=private-work-chat-test")
			require.Equal(t, "private-work-chat-test", response.Header().Get("X-Request-ID"))
			for _, private := range []string{"private-response-chat", "private-request-chat", "private-signed-url", "private-cursor", "private-token", "http.request_body", "http.response_body", "http.query_params"} {
				require.NotContains(t, logs.String(), private)
			}
		})
	}
}

func TestRequestLoggerStillCapturesOrdinaryCategoryPostPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	router.POST("/categories", func(c *gin.Context) {
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"name": "public-category-response"})
	})
	request := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader(`{"name":"public-category-request"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "public-category-test")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Contains(t, logs.String(), "public-category-request")
	require.Contains(t, logs.String(), "public-category-response")
	require.Contains(t, logs.String(), "http.request_body")
	require.Contains(t, logs.String(), "http.response_body")
	require.Contains(t, logs.String(), "http.method=POST")
	require.Contains(t, logs.String(), "http.status_code=201")
	require.Contains(t, logs.String(), "request_id=public-category-test")
}
