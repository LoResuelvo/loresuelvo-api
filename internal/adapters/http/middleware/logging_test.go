package middleware

import (
	"io"
	"log/slog"
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
