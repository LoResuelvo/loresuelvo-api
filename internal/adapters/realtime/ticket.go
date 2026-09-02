package realtime

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// PostgresTicketStore persists one-time WebSocket authentication tickets.
// PostgreSQL is the authority for expiration and atomic consumption across
// every API instance.
type PostgresTicketStore struct {
	db *sql.DB
}

// NewPostgresTicketStore creates a ticket store backed by PostgreSQL. Tickets
// written through this store can be issued and consumed by any API instance
// connected to the same database.
func NewPostgresTicketStore(db *sql.DB) *PostgresTicketStore {
	return &PostgresTicketStore{db: db}
}

// Issue generates and persists a secure ticket with a one-minute lifetime.
func (s *PostgresTicketStore) Issue(ctx context.Context, authID string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("issuing websocket ticket: database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating websocket ticket: %w", err)
	}
	ticket := hex.EncodeToString(bytes)

	_, err := s.db.ExecContext(
		ctx,
		`WITH expired AS (
			DELETE FROM websocket_tickets WHERE expires_at <= NOW()
		)
		INSERT INTO websocket_tickets (ticket, auth_id, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 minute')`,
		ticket,
		authID,
	)
	if err != nil {
		return "", fmt.Errorf("persisting websocket ticket: %w", err)
	}
	return ticket, nil
}

// Consume atomically consumes a ticket. PostgreSQL performs the
// validation and deletion in one statement, so concurrent API instances
// cannot successfully consume the same ticket.
func (s *PostgresTicketStore) Consume(ctx context.Context, ticket string) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("consuming websocket ticket: database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var authID string
	err := s.db.QueryRowContext(
		ctx,
		`DELETE FROM websocket_tickets
		WHERE ticket = $1 AND expires_at > NOW()
		RETURNING auth_id`,
		ticket,
	).Scan(&authID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("consuming websocket ticket: %w", err)
	}
	return authID, true, nil
}
