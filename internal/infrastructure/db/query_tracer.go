package db

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type queryTraceContextKey struct{}

type queryTrace struct {
	startedOn   time.Time
	operation   string
	fingerprint string
}

type QueryTracer struct {
	logger *slog.Logger
}

func NewQueryTracer(logger *slog.Logger) *QueryTracer {
	if logger == nil {
		logger = slog.Default()
	}
	return &QueryTracer{logger: logger}
}

func (tracer *QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	trace := queryTrace{
		startedOn:   time.Now(),
		operation:   databaseOperation(data.SQL),
		fingerprint: queryFingerprint(data.SQL),
	}
	return context.WithValue(ctx, queryTraceContextKey{}, trace)
}

func (tracer *QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	trace, ok := ctx.Value(queryTraceContextKey{}).(queryTrace)
	if !ok {
		trace = queryTrace{startedOn: time.Now(), operation: "UNKNOWN", fingerprint: "unknown"}
	}

	logger := observability.LoggerFromContextOr(ctx, tracer.logger)
	attributes := []any{
		"db.system", "postgresql",
		"db.operation", trace.operation,
		"db.query_fingerprint", trace.fingerprint,
		"db.rows_affected", data.CommandTag.RowsAffected(),
		"duration_ms", time.Since(trace.startedOn).Milliseconds(),
	}

	if data.Err == nil {
		logger.DebugContext(ctx, "db.query.completed", attributes...)
		return
	}

	attributes = append(attributes, safeDatabaseErrorAttributes(data.Err)...)
	if errors.Is(data.Err, context.Canceled) || errors.Is(data.Err, context.DeadlineExceeded) {
		logger.WarnContext(ctx, "db.query.completed", attributes...)
		return
	}
	logger.ErrorContext(ctx, "db.query.completed", attributes...)
}

func databaseOperation(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return "UNKNOWN"
	}
	return strings.ToUpper(fields[0])
}

func queryFingerprint(query string) string {
	normalized := strings.Join(strings.Fields(query), " ")
	digest := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", digest[:8])
}

func safeDatabaseErrorAttributes(err error) []any {
	attributes := []any{"error.type", fmt.Sprintf("%T", err)}
	if postgresError, ok := errors.AsType[*pgconn.PgError](err); ok {
		attributes = append(attributes,
			"db.error_code", postgresError.Code,
			"db.error_message", postgresError.Message,
		)
	}
	return attributes
}
