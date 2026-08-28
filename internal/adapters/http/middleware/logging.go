package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestIDFromHeader(c.GetHeader(requestIDHeader))
		requestLogger := logger.With("request_id", requestID)
		c.Request = c.Request.WithContext(observability.ContextWithLogger(c.Request.Context(), requestLogger))
		c.Header(requestIDHeader, requestID)

		includeBodies := requestLogger.Enabled(c.Request.Context(), slog.LevelInfo)
		var requestBody *limitedBodyCapture
		var responseBody *limitedBodyCapture
		if includeBodies {
			requestBody = captureRequestBody(c.Request)
			responseWriter, capture := captureResponseBody(c.Writer)
			c.Writer = responseWriter
			responseBody = capture
		}

		startedOn := time.Now()
		c.Next()

		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		if c.Request.Method == http.MethodGet && route == "/" && status < http.StatusBadRequest {
			return
		}

		attributes := []any{
			"http.method", c.Request.Method,
			"http.route", route,
			"http.status_code", status,
			"duration_ms", time.Since(startedOn).Milliseconds(),
		}
		if pathParams := pathParameters(c.Params); len(pathParams) > 0 {
			attributes = append(attributes, "http.path_params", pathParams)
		}
		if queryParams := queryParameters(c.Request); len(queryParams) > 0 {
			attributes = append(attributes, "http.query_params", queryParams)
		}
		if includeBodies && routeAllowsBodyLogging(c.Request.Method, c.FullPath()) {
			attributes = append(attributes, capturedBodyAttributes(
				"http.request",
				requestBody,
				c.Request.Header.Get("Content-Type"),
			)...)
			attributes = append(attributes, capturedBodyAttributes(
				"http.response",
				responseBody,
				c.Writer.Header().Get("Content-Type"),
			)...)
		}
		requestLogger.Log(c.Request.Context(), httpLogLevel(status), "http.request.completed", attributes...)
	}
}

func routeAllowsBodyLogging(method, route string) bool {
	return method != http.MethodPost || route != "/providers/me/identity-verification-sessions"
}

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestLogger := observability.LoggerFromContextOr(c.Request.Context(), logger)
				requestLogger.ErrorContext(c.Request.Context(), "http.request.panicked",
					"panic_type", fmt.Sprintf("%T", recovered),
					"stack", safeStackTrace(),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func requestIDFromHeader(header string) string {
	header = strings.TrimSpace(header)
	if validRequestID.MatchString(header) {
		return header
	}
	return uuid.NewString()
}

func httpLogLevel(status int) slog.Level {
	if status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	return slog.LevelInfo
}

func pathParameters(params gin.Params) map[string]string {
	result := make(map[string]string, len(params))
	for _, param := range params {
		result[param.Key] = param.Value
	}
	return result
}

func queryParameters(request *http.Request) map[string]string {
	result := make(map[string]string)
	for _, key := range []string{"category_id", "data.id"} {
		if value := strings.TrimSpace(request.URL.Query().Get(key)); value != "" {
			result[key] = value
		}
	}
	return result
}

func safeStackTrace() string {
	programCounters := make([]uintptr, 32)
	count := runtime.Callers(4, programCounters)
	frames := runtime.CallersFrames(programCounters[:count])

	var stack strings.Builder
	for {
		frame, more := frames.Next()
		if frame.Function != "" {
			if stack.Len() > 0 {
				stack.WriteString(" <- ")
			}
			stack.WriteString(frame.Function)
			stack.WriteString(":")
			stack.WriteString(strconv.Itoa(frame.Line))
		}
		if !more {
			break
		}
	}
	return stack.String()
}
