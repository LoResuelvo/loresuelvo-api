package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const serviceName = "loresuelvo-api"

type Config struct {
	Environment string
	Level       slog.Level
}

func NewConfigFromEnv() (Config, error) {
	level, err := parseLevel(envOrDefault("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Environment: envOrDefault("ENVIRONMENT", "dev"),
		Level:       level,
	}, nil
}

func NewLogger(config Config, output io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: config.Level})
	return slog.New(handler).With(
		"service", serviceName,
		"env", config.Environment,
	)
}

func NewLoggerFromEnv() (*slog.Logger, error) {
	config, err := NewConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewLogger(config, os.Stdout), nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: expected debug, info, warn, or error", value)
	}
}

func envOrDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}
