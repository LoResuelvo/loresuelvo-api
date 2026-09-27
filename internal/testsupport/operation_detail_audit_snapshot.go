package testsupport

import (
	"context"
	"database/sql"
	"fmt"
)

// OperationDetailBusinessCounts captures the persisted row counts of the
// business tables that an administrative detail read must never mutate.
func OperationDetailBusinessCounts(ctx context.Context, db *sql.DB) (map[string]int64, error) {
	tables := [...]string{
		"job_requests", "service_proposals", "work_orders", "payment_intents",
		"payment_transactions", "conversations", "messages",
	}
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		var count int64
		// Table names come only from the constant list above, never from scenario input.
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("counting %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}

// OperationDetailAuditCorrelationCount checks all persisted events rather than
// relying on a bounded audit page to establish correlation uniqueness.
func OperationDetailAuditCorrelationCount(ctx context.Context, db *sql.DB, correlation string) (int64, error) {
	var count int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE correlation_id = $1", correlation).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting audit correlation: %w", err)
	}
	return count, nil
}

// ResetOperationDetailAuditCorrelation removes only prior test-run evidence for
// this fixed Gherkin correlation. Production audit repositories remain append-only.
func ResetOperationDetailAuditCorrelation(ctx context.Context, db *sql.DB, correlation string) error {
	if _, err := db.ExecContext(ctx, "DELETE FROM audit_events WHERE correlation_id = $1", correlation); err != nil {
		return fmt.Errorf("resetting test audit correlation: %w", err)
	}
	return nil
}
