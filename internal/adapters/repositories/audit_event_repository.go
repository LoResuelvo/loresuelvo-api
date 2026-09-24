package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
)

// AuditEventRepository persists immutable audit events. It intentionally has no
// operation that changes or removes an existing event.
type AuditEventRepository struct{ db *sql.DB }

type auditEventExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func NewAuditEventRepository(db *sql.DB) *AuditEventRepository {
	return &AuditEventRepository{db: db}
}

func (repository *AuditEventRepository) Save(ctx context.Context, event *audit.Event) error {
	return repository.saveWithExecutor(ctx, repository.db, event)
}

// saveWithExecutor accepts either the connection pool or a caller-owned
// transaction. It never starts, commits, or rolls back the caller's transaction.
func (repository *AuditEventRepository) saveWithExecutor(ctx context.Context, executor auditEventExecutor, event *audit.Event) error {
	if event == nil {
		return fmt.Errorf("saving audit event: %w", audit.ErrInvalidEvent)
	}

	var reason, changedField, stateFrom, stateTo any
	if value := event.Reason(); value != nil {
		reason = value.Text()
	}
	if value := event.StateChange(); value != nil {
		changedField, stateFrom, stateTo = value.Field(), value.From(), value.To()
	}

	_, err := executor.ExecContext(ctx, `
		INSERT INTO audit_events (
			id, operator_id, action, resource_type, resource_id, occurred_on,
			result, correlation_id, reason, changed_field, state_from, state_to
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		event.ID(), event.OperatorID(), event.Action(), event.ResourceType(),
		event.ResourceID(), event.OccurredOn(), event.Result(), event.CorrelationID(),
		reason, changedField, stateFrom, stateTo)
	if err != nil {
		return fmt.Errorf("saving audit event: %w: %w", audit.ErrPersistence, err)
	}
	return nil
}

func (repository *AuditEventRepository) FindByID(ctx context.Context, id uuid.UUID) (*audit.Event, error) {
	event, err := scanAuditEvent(repository.db.QueryRowContext(ctx, `
		SELECT id, operator_id, action, resource_type, resource_id, occurred_on,
			result, correlation_id, reason, changed_field, state_from, state_to
		FROM audit_events WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, audit.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding audit event: %w: %w", audit.ErrPersistence, err)
	}
	return event, nil
}

// FindLatest returns only a bounded, deterministic, newest-first selection.
func (repository *AuditEventRepository) FindLatest(ctx context.Context, filter audit.LogFilter, limit int) ([]*audit.Event, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("finding latest audit events: %w", audit.ErrInvalidQuery)
	}

	const columns = `SELECT id, operator_id, action, resource_type, resource_id, occurred_on,
		result, correlation_id, reason, changed_field, state_from, state_to FROM audit_events`
	conditions := make([]string, 0, 7)
	args := make([]any, 0, 8)
	appendCondition := func(column, comparison string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s %s $%d", column, comparison, len(args)))
	}
	if filter.OperatorID != nil {
		appendCondition("operator_id", "=", *filter.OperatorID)
	}
	if filter.Action != nil {
		appendCondition("action", "=", *filter.Action)
	}
	if filter.ResourceType != nil {
		appendCondition("resource_type", "=", *filter.ResourceType)
	}
	if filter.ResourceID != nil {
		appendCondition("resource_id", "=", *filter.ResourceID)
	}
	if filter.Result != nil {
		appendCondition("result", "=", *filter.Result)
	}
	if filter.OccurredFrom != nil {
		appendCondition("occurred_on", ">=", filter.OccurredFrom.UTC())
	}
	if filter.OccurredTo != nil {
		appendCondition("occurred_on", "<", filter.OccurredTo.UTC())
	}
	query := columns
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}
	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY occurred_on DESC, id DESC LIMIT $%d`, len(args))
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("finding latest audit events: %w: %w", audit.ErrPersistence, err)
	}
	defer rows.Close()

	events := make([]*audit.Event, 0, limit)
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("rehydrating latest audit event: %w: %w", audit.ErrPersistence, err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating latest audit events: %w: %w", audit.ErrPersistence, err)
	}
	return events, nil
}

type auditEventScanner interface {
	Scan(dest ...any) error
}

func scanAuditEvent(row auditEventScanner) (*audit.Event, error) {
	var id uuid.UUID
	var operatorID int
	var action audit.Action
	var resourceType, resourceID string
	var occurredOn time.Time
	var result audit.Result
	var correlationID string
	var reasonText, changedField, stateFrom, stateTo sql.NullString

	err := row.Scan(&id, &operatorID, &action, &resourceType, &resourceID, &occurredOn,
		&result, &correlationID, &reasonText, &changedField, &stateFrom, &stateTo)
	if err != nil {
		return nil, err
	}

	var reason *audit.Reason
	if reasonText.Valid {
		reason, err = audit.NewReason(reasonText.String)
		if err != nil {
			return nil, fmt.Errorf("rehydrating audit event reason: %w", err)
		}
	}
	var stateChange *audit.StateChange
	if changedField.Valid || stateFrom.Valid || stateTo.Valid {
		stateChange, err = audit.NewStateChange(changedField.String, stateFrom.String, stateTo.String)
		if err != nil {
			return nil, fmt.Errorf("rehydrating audit event state change: %w", err)
		}
	}

	event, err := audit.NewEvent(audit.EventParams{
		ID: id, OperatorID: operatorID, Action: action, ResourceType: resourceType,
		ResourceID: resourceID, OccurredOn: occurredOn, Result: result,
		CorrelationID: correlationID, Reason: reason, StateChange: stateChange,
	})
	if err != nil {
		return nil, fmt.Errorf("rehydrating audit event: %w", err)
	}
	return event, nil
}
