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

func TestRequestLoggerSuppressesReviewReportContents(t *testing.T) {
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
	router.POST("/work-orders/:workOrderID/reviews/reports", func(c *gin.Context) {
		data, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Contains(t, string(data), "secret explanation")
		c.JSON(500, gin.H{"error": "internal server error"})
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/work-orders/1/reviews/reports", strings.NewReader(`{"explanation":"secret explanation"}`)))
	require.Contains(t, logs.String(), "http.request.completed")
	require.NotContains(t, logs.String(), "secret explanation")
	require.NotContains(t, logs.String(), "http.request_body")
	require.NotContains(t, logs.String(), "http.response_body")
}
