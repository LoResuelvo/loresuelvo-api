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

		startedOn := time.Now()
		c.Next()

		status := c.Writer.Status()
		responseSize := max(c.Writer.Size(), 0)
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		level := httpLogLevel(status)
		if c.Request.Method == http.MethodGet && route == "/" && status < http.StatusBadRequest {
			level = slog.LevelDebug
		}
		requestLogger.Log(c.Request.Context(), level, "http.request.completed",
			"http.method", c.Request.Method,
			"http.route", route,
			"http.status_code", status,
			"http.response_size_bytes", responseSize,
			"duration_ms", time.Since(startedOn).Milliseconds(),
		)
	}
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
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
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
