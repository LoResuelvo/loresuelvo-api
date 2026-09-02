package realtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	// RealtimeEventsChannel is the PostgreSQL channel shared by all API
	// instances that participate in realtime delivery.
	RealtimeEventsChannel = "loresuelvo_realtime_events"

	eventBusInitialReconnectDelay = 100 * time.Millisecond
	eventBusMaximumReconnectDelay = 5 * time.Second
)

// EventEnvelope is the transport representation of a realtime event. The
// payload is kept as the exact JSON document sent to a WebSocket client while
// the target fields let each API instance deliver it to its local connections.
type EventEnvelope struct {
	ID              string          `json:"id"`
	TargetAuthID    string          `json:"target_auth_id"`
	TargetRole      string          `json:"target_role"`
	TargetProfileID int             `json:"target_profile_id"`
	Payload         json.RawMessage `json:"payload"`
}

// EventBus is the transport capability required by Dispatcher.
// Dispatchers call it after the business transaction has committed and listen
// independently on every API instance.
type EventBus interface {
	Publish(context.Context, EventEnvelope) error
	Listen(context.Context, func(EventEnvelope)) error
}

// PostgresEventBus stores envelopes in PostgreSQL and uses LISTEN/NOTIFY only
// as a wake-up signal. Sending only the event ID through NOTIFY avoids its
// payload size limit while preserving the exact WebSocket payload.
type PostgresEventBus struct {
	db      *sql.DB
	channel string
	logger  *slog.Logger
}

// NewPostgresEventBus creates a PostgreSQL-backed realtime event bus.
func NewPostgresEventBus(db *sql.DB) *PostgresEventBus {
	return &PostgresEventBus{
		db:      db,
		channel: RealtimeEventsChannel,
		logger:  slog.Default(),
	}
}

// Publish stores an event and signals every listening API instance in one
// transaction. Callers invoke it after the business transaction has committed.
func (bus *PostgresEventBus) Publish(ctx context.Context, event EventEnvelope) error {
	if bus == nil || bus.db == nil {
		return fmt.Errorf("publishing realtime event: event bus database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateEventEnvelope(event); err != nil {
		return fmt.Errorf("publishing realtime event: %w", err)
	}

	tx, err := bus.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning realtime event transaction: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM realtime_events WHERE created_at <= NOW() - INTERVAL '1 hour'`); err != nil {
		return rollbackEventBusTx(tx, fmt.Errorf("cleaning realtime events: %w", err))
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO realtime_events (id, target_auth_id, target_role, target_profile_id, payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING`,
		event.ID,
		event.TargetAuthID,
		event.TargetRole,
		event.TargetProfileID,
		[]byte(event.Payload),
	); err != nil {
		return rollbackEventBusTx(tx, fmt.Errorf("storing realtime event: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_notify($1, $2)`, bus.channelName(), event.ID); err != nil {
		return rollbackEventBusTx(tx, fmt.Errorf("signaling realtime event: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing realtime event transaction: %w", err)
	}
	return nil
}

// Listen waits for events and invokes handler for each valid envelope. It
// reconnects when PostgreSQL closes the dedicated listener connection and
// stops promptly when ctx is canceled.
func (bus *PostgresEventBus) Listen(ctx context.Context, handler func(EventEnvelope)) error {
	if bus == nil || bus.db == nil {
		return fmt.Errorf("listening for realtime events: event bus database is required")
	}
	if handler == nil {
		return fmt.Errorf("listening for realtime events: handler is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	delay := eventBusInitialReconnectDelay
	for {
		err := bus.listenOnce(ctx, handler)
		if err == nil {
			err = errors.New("realtime listener stopped")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		bus.loggerOrDefault().Warn("realtime event listener disconnected; retrying",
			"error", err, "retry_in", delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
		if delay < eventBusMaximumReconnectDelay {
			delay *= 2
			if delay > eventBusMaximumReconnectDelay {
				delay = eventBusMaximumReconnectDelay
			}
		}
	}
}

func (bus *PostgresEventBus) listenOnce(ctx context.Context, handler func(EventEnvelope)) error {
	connection, err := bus.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("opening realtime listener connection: %w", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			bus.loggerOrDefault().Warn("closing realtime listener connection failed", "error", closeErr)
		}
	}()

	err = connection.Raw(func(driverConnection any) error {
		stdlibConnection, ok := driverConnection.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("opening realtime listener connection: unsupported driver %T", driverConnection)
		}

		pgxConnection := stdlibConnection.Conn()
		if _, err := pgxConnection.Exec(ctx, `LISTEN `+pgx.Identifier{bus.channelName()}.Sanitize()); err != nil {
			return fmt.Errorf("subscribing to realtime events: %w", err)
		}

		for {
			notification, err := pgxConnection.WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("waiting for realtime event: %w", err)
			}
			if notification == nil || notification.Payload == "" {
				continue
			}

			event, err := bus.findEvent(ctx, notification.Payload)
			if errors.Is(err, sql.ErrNoRows) {
				bus.loggerOrDefault().Warn("ignoring missing realtime event", "eventID", notification.Payload)
				continue
			}
			if err != nil {
				return err
			}
			if err := validateEventEnvelope(event); err != nil {
				bus.loggerOrDefault().Warn("ignoring invalid realtime event", "eventID", notification.Payload, "error", err)
				continue
			}
			handler(event)
		}
	})
	if err != nil {
		return err
	}
	return nil
}

func (bus *PostgresEventBus) findEvent(ctx context.Context, id string) (EventEnvelope, error) {
	var event EventEnvelope
	err := bus.db.QueryRowContext(
		ctx,
		`SELECT id, target_auth_id, target_role, target_profile_id, payload
		FROM realtime_events
		WHERE id = $1`,
		id,
	).Scan(&event.ID, &event.TargetAuthID, &event.TargetRole, &event.TargetProfileID, &event.Payload)
	if err != nil {
		return EventEnvelope{}, fmt.Errorf("loading realtime event %q: %w", id, err)
	}
	return event, nil
}

func rollbackEventBusTx(tx *sql.Tx, cause error) error {
	if err := tx.Rollback(); err != nil {
		return errors.Join(cause, fmt.Errorf("rolling back realtime event transaction: %w", err))
	}
	return cause
}

func (bus *PostgresEventBus) channelName() string {
	if bus.channel != "" {
		return bus.channel
	}
	return RealtimeEventsChannel
}

func (bus *PostgresEventBus) loggerOrDefault() *slog.Logger {
	if bus.logger != nil {
		return bus.logger
	}
	return slog.Default()
}

func validateEventEnvelope(event EventEnvelope) error {
	if event.ID == "" {
		return fmt.Errorf("event id is required")
	}
	if event.TargetAuthID == "" {
		return fmt.Errorf("event target auth id is required")
	}
	if event.TargetRole == "" {
		return fmt.Errorf("event target role is required")
	}
	if event.TargetProfileID <= 0 {
		return fmt.Errorf("event target profile id is required")
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return fmt.Errorf("event payload must be valid JSON")
	}
	return nil
}
