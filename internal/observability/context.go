package observability

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	return LoggerFromContextOr(ctx, slog.Default())
}

func LoggerFromContextOr(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return fallback
}
