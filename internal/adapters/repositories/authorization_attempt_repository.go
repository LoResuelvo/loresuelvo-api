package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type AuthorizationAttemptRepository struct {
	db *sql.DB
}

func NewAuthorizationAttemptRepository(db *sql.DB) *AuthorizationAttemptRepository {
	return &AuthorizationAttemptRepository{db: db}
}

func (repository *AuthorizationAttemptRepository) Save(ctx context.Context, attempt *paymentaccount.AuthorizationAttempt) error {
	err := repository.db.QueryRowContext(
		ctx,
		`INSERT INTO payment_account_authorization_attempts
		(provider_id, payment_provider, state_digest, code_verifier_ciphertext, expires_on)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		attempt.ProviderID,
		attempt.PaymentProvider,
		attempt.StateDigest,
		attempt.CodeVerifierCiphertext,
		attempt.ExpiresOn,
	).Scan(&attempt.ID)
	if err != nil {
		return fmt.Errorf("saving payment account authorization attempt: %w", err)
	}
	return nil
}

func (repository *AuthorizationAttemptRepository) FindByStateDigest(ctx context.Context, stateDigest []byte) (*paymentaccount.AuthorizationAttempt, error) {
	var attempt paymentaccount.AuthorizationAttempt
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, provider_id, payment_provider, state_digest, code_verifier_ciphertext, expires_on
		FROM payment_account_authorization_attempts
		WHERE state_digest = $1 AND consumed_on IS NULL`,
		stateDigest,
	).Scan(
		&attempt.ID,
		&attempt.ProviderID,
		&attempt.PaymentProvider,
		&attempt.StateDigest,
		&attempt.CodeVerifierCiphertext,
		&attempt.ExpiresOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, paymentaccount.ErrAuthorizationAttemptNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding payment account authorization attempt: %w", err)
	}
	attempt.ExpiresOn = attempt.ExpiresOn.UTC()
	return &attempt, nil
}

func (repository *AuthorizationAttemptRepository) Consume(ctx context.Context, attempt *paymentaccount.AuthorizationAttempt) error {
	return consumeAuthorizationAttempt(ctx, repository.db, attempt.ID, attempt.ProviderID, attempt.PaymentProvider)
}

func (repository *AuthorizationAttemptRepository) markConsumedWithTx(ctx context.Context, tx *sql.Tx, attemptID, providerID int, paymentProvider paymentaccount.PaymentProvider) error {
	return consumeAuthorizationAttempt(ctx, tx, attemptID, providerID, paymentProvider)
}

type contextExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func consumeAuthorizationAttempt(ctx context.Context, executor contextExecutor, attemptID, providerID int, paymentProvider paymentaccount.PaymentProvider) error {
	result, err := executor.ExecContext(
		ctx,
		`UPDATE payment_account_authorization_attempts
		SET consumed_on = NOW()
		WHERE id = $1 AND provider_id = $2 AND payment_provider = $3 AND consumed_on IS NULL`,
		attemptID,
		providerID,
		paymentProvider,
	)
	if err != nil {
		return fmt.Errorf("consuming payment account authorization attempt: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking consumed payment account authorization attempt: %w", err)
	}
	if rowsAffected != 1 {
		return paymentaccount.ErrAuthorizationAttemptNotFound
	}
	return nil
}

func (repository *AuthorizationAttemptRepository) deleteAllWithTx(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM payment_account_authorization_attempts`); err != nil {
		return fmt.Errorf("deleting payment account authorization attempts: %w", err)
	}
	return nil
}
