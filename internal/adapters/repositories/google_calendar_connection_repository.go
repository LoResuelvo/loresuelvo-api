package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
)

type GoogleCalendarConnectionRepository struct {
	db                             *sql.DB
	authorizationAttemptRepository *GoogleCalendarAuthorizationAttemptRepository
}

func NewGoogleCalendarConnectionRepository(
	db *sql.DB,
	authorizationAttemptRepository *GoogleCalendarAuthorizationAttemptRepository,
) *GoogleCalendarConnectionRepository {
	return &GoogleCalendarConnectionRepository{
		db:                             db,
		authorizationAttemptRepository: authorizationAttemptRepository,
	}
}

func (repository *GoogleCalendarConnectionRepository) SaveFromAuthorization(
	ctx context.Context,
	attemptID int,
	connection *calendarconnection.Connection,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning calendar connection transaction: %w", err)
	}
	if err := repository.saveWithTx(ctx, tx, connection); err != nil {
		return rollbackCalendarConnectionTx(tx, err)
	}
	if err := repository.authorizationAttemptRepository.markConsumedWithTx(ctx, tx, attemptID); err != nil {
		return rollbackCalendarConnectionTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing calendar connection transaction: %w", err)
	}
	return nil
}

func (repository *GoogleCalendarConnectionRepository) saveWithTx(ctx context.Context, tx *sql.Tx, connection *calendarconnection.Connection) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO google_calendar_connections
		(user_id, refresh_token_ciphertext, calendar_id, status, connected_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
		refresh_token_ciphertext = EXCLUDED.refresh_token_ciphertext,
		calendar_id = EXCLUDED.calendar_id,
		status = EXCLUDED.status,
		connected_on = EXCLUDED.connected_on,
		updated_on = EXCLUDED.updated_on`,
		connection.UserID(),
		connection.RefreshTokenCiphertext(),
		connection.CalendarID(),
		connection.Status(),
		connection.ConnectedOn(),
		connection.UpdatedOn(),
	)
	if err != nil {
		return fmt.Errorf("saving calendar connection: %w", err)
	}
	return nil
}

func (repository *GoogleCalendarConnectionRepository) FindByUserID(ctx context.Context, userID int) (*calendarconnection.Connection, error) {
	var (
		refreshTokenCiphertext []byte
		calendarID             string
		status                 string
		connectedOn            = sql.NullTime{}
		updatedOn              = sql.NullTime{}
	)
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT refresh_token_ciphertext, calendar_id, status, connected_on, updated_on
		FROM google_calendar_connections
		WHERE user_id = $1`,
		userID,
	).Scan(&refreshTokenCiphertext, &calendarID, &status, &connectedOn, &updatedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, calendarconnection.ErrConnectionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding calendar connection: %w", err)
	}
	connection, err := calendarconnection.RehydrateConnection(
		userID,
		refreshTokenCiphertext,
		calendarID,
		status,
		connectedOn.Time,
		updatedOn.Time,
	)
	if err != nil {
		return nil, fmt.Errorf("restoring calendar connection: %w", err)
	}
	return connection, nil
}

func (repository *GoogleCalendarConnectionRepository) DeleteAll() error {
	tx, err := repository.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning calendar connection cleanup transaction: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM google_calendar_connections`); err != nil {
		return rollbackCalendarConnectionTx(tx, fmt.Errorf("deleting calendar connections: %w", err))
	}
	if err := repository.authorizationAttemptRepository.deleteAllWithTx(tx); err != nil {
		return rollbackCalendarConnectionTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing calendar connection cleanup transaction: %w", err)
	}
	return nil
}

func rollbackCalendarConnectionTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback calendar connection transaction: %v", originalErr, rollbackErr)
	}
	return originalErr
}
