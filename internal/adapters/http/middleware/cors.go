package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const corsAllowedOriginsEnv = "CORS_ALLOWED_ORIGINS"

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

func NewCORSConfigFromEnv() CORSConfig {
	return CORSConfig{
		AllowedOrigins: splitCommaSeparated(os.Getenv(corsAllowedOriginsEnv)),
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}
}

func CORSLayer(config CORSConfig) gin.HandlerFunc {
	allowedOrigins := map[string]struct{}{}
	allowAnyOrigin := false
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			allowAnyOrigin = true
			continue
		}
		allowedOrigins[origin] = struct{}{}
	}

	allowedMethods := strings.Join(config.AllowedMethods, ", ")
	allowedHeaders := strings.Join(config.AllowedHeaders, ", ")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAnyOrigin || isAllowedOrigin(origin, allowedOrigins)) {
			allowOrigin := origin
			if allowAnyOrigin {
				allowOrigin = "*"
			}

			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", allowOrigin)
			header.Set("Access-Control-Allow-Methods", allowedMethods)
			header.Set("Access-Control-Allow-Headers", allowedHeaders)
			header.Add("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isAllowedOrigin(origin string, allowedOrigins map[string]struct{}) bool {
	_, ok := allowedOrigins[origin]
	return ok
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
