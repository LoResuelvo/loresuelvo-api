package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type PaymentTransactionRepository struct {
	db *sql.DB
}

const paymentTransactionAdvisoryLockNamespace = 2120

func NewPaymentTransactionRepository(db *sql.DB) *PaymentTransactionRepository {
	return &PaymentTransactionRepository{db: db}
}

func (repository *PaymentTransactionRepository) WithinPaymentLock(
	ctx context.Context,
	processor paymentaccount.PaymentProvider,
	externalPaymentID string,
	operation func() error,
) error {
	if processor == "" || externalPaymentID == "" || operation == nil {
		return fmt.Errorf("locking external payment: processor, external payment, and operation are required")
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning external payment lock transaction: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock($1, hashtext($2))`,
		paymentTransactionAdvisoryLockNamespace,
		string(processor)+":"+externalPaymentID,
	); err != nil {
		return rollbackPaymentTransactionTx(tx, fmt.Errorf("locking external payment: %w", err))
	}
	if err := operation(); err != nil {
		return rollbackPaymentTransactionTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing external payment lock transaction: %w", err)
	}
	return nil
}

func (repository *PaymentTransactionRepository) Exists(
	ctx context.Context,
	processor paymentaccount.PaymentProvider,
	externalPaymentID string,
) (bool, error) {
	var exists bool
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM payment_transactions
			WHERE processor = $1 AND external_payment_id = $2
		)`,
		processor,
		externalPaymentID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking payment transaction existence: %w", err)
	}
	return exists, nil
}

func (repository *PaymentTransactionRepository) saveWithTx(
	ctx context.Context,
	tx *sql.Tx,
	transaction *payment.Transaction,
) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("saving payment transaction: transaction boundary is required")
	}
	if transaction == nil {
		return false, fmt.Errorf("saving payment transaction: payment transaction is required")
	}

	var id int
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO payment_transactions (
			payment_intent_id,
			processor,
			external_payment_id,
			seller_account_id,
			status,
			currency,
			amount_cents,
			verified_on,
			created_on,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (processor, external_payment_id) DO NOTHING
		RETURNING id`,
		transaction.PaymentIntentID,
		transaction.Processor,
		transaction.ExternalPaymentID,
		transaction.SellerAccountID,
		transaction.Status,
		transaction.Currency,
		transaction.AmountCents,
		transaction.VerifiedOn,
		transaction.CreatedOn,
		transaction.UpdatedOn,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("saving payment transaction: %w", err)
	}
	transaction.ID = id
	return true, nil
}

func (repository *PaymentTransactionRepository) CountByExternalPaymentID(
	ctx context.Context,
	processor paymentaccount.PaymentProvider,
	externalPaymentID string,
) (int, error) {
	var count int
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM payment_transactions
		WHERE processor = $1 AND external_payment_id = $2`,
		processor,
		externalPaymentID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting external payment transactions: %w", err)
	}
	return count, nil
}

func rollbackPaymentTransactionTx(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back payment transaction: %v", cause, rollbackErr)
	}
	return cause
}
