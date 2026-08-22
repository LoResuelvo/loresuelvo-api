package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
)

type GoogleCalendarAuthorizationAttemptRepository struct {
	db *sql.DB
}

func NewGoogleCalendarAuthorizationAttemptRepository(db *sql.DB) *GoogleCalendarAuthorizationAttemptRepository {
	return &GoogleCalendarAuthorizationAttemptRepository{db: db}
}

func (repository *GoogleCalendarAuthorizationAttemptRepository) Save(ctx context.Context, attempt *calendarconnection.AuthorizationAttempt) error {
	err := repository.db.QueryRowContext(
		ctx,
		`INSERT INTO google_calendar_authorization_attempts
		(user_id, state_digest, code_verifier_ciphertext, expires_on)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		attempt.UserID,
		attempt.StateDigest,
		attempt.CodeVerifierCiphertext,
		attempt.ExpiresOn,
	).Scan(&attempt.ID)
	if err != nil {
		return fmt.Errorf("saving calendar authorization attempt: %w", err)
	}
	return nil
}

func (repository *GoogleCalendarAuthorizationAttemptRepository) FindByStateDigest(ctx context.Context, stateDigest []byte) (*calendarconnection.AuthorizationAttempt, error) {
	var (
		attempt    calendarconnection.AuthorizationAttempt
		consumedOn sql.NullTime
	)
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, state_digest, code_verifier_ciphertext, expires_on, consumed_on
		FROM google_calendar_authorization_attempts
		WHERE state_digest = $1`,
		stateDigest,
	).Scan(
		&attempt.ID,
		&attempt.UserID,
		&attempt.StateDigest,
		&attempt.CodeVerifierCiphertext,
		&attempt.ExpiresOn,
		&consumedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, calendarconnection.ErrAuthorizationAttemptNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding calendar authorization attempt: %w", err)
	}
	if consumedOn.Valid {
		value := consumedOn.Time.UTC()
		attempt.ConsumedOn = &value
	}
	attempt.ExpiresOn = attempt.ExpiresOn.UTC()
	return &attempt, nil
}

func (repository *GoogleCalendarAuthorizationAttemptRepository) Consume(ctx context.Context, attempt *calendarconnection.AuthorizationAttempt) error {
	return consumeGoogleCalendarAuthorizationAttempt(ctx, repository.db, attempt.ID)
}

func (repository *GoogleCalendarAuthorizationAttemptRepository) markConsumedWithTx(ctx context.Context, tx *sql.Tx, attemptID int) error {
	return consumeGoogleCalendarAuthorizationAttempt(ctx, tx, attemptID)
}

type googleCalendarContextExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func consumeGoogleCalendarAuthorizationAttempt(ctx context.Context, executor googleCalendarContextExecutor, attemptID int) error {
	result, err := executor.ExecContext(
		ctx,
		`UPDATE google_calendar_authorization_attempts
		SET consumed_on = NOW()
		WHERE id = $1 AND consumed_on IS NULL`,
		attemptID,
	)
	if err != nil {
		return fmt.Errorf("consuming calendar authorization attempt: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking consumed calendar authorization attempt: %w", err)
	}
	if rowsAffected == 1 {
		return nil
	}

	var exists bool
	if err := executor.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM google_calendar_authorization_attempts WHERE id = $1)`,
		attemptID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("checking calendar authorization attempt: %w", err)
	}
	if !exists {
		return calendarconnection.ErrAuthorizationAttemptNotFound
	}
	return calendarconnection.ErrAuthorizationAttemptConsumed
}

func (repository *GoogleCalendarAuthorizationAttemptRepository) deleteAllWithTx(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM google_calendar_authorization_attempts`); err != nil {
		return fmt.Errorf("deleting calendar authorization attempts: %w", err)
	}
	return nil
}
