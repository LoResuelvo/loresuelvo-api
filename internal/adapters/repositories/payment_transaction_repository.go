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

func NewPaymentTransactionRepository(db *sql.DB) *PaymentTransactionRepository {
	return &PaymentTransactionRepository{db: db}
}

func (repository *PaymentTransactionRepository) Save(
	ctx context.Context,
	transaction *payment.Transaction,
) error {
	if transaction == nil {
		return fmt.Errorf("saving payment transaction: payment transaction is required")
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning payment transaction: %w", err)
	}
	if err := repository.saveWithTx(ctx, tx, transaction); err != nil {
		return rollbackPaymentTransactionTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing payment transaction: %w", err)
	}
	return nil
}

func (repository *PaymentTransactionRepository) FindByExternalID(
	ctx context.Context,
	processor paymentaccount.PaymentProvider,
	externalPaymentID string,
) (*payment.Transaction, error) {
	var transaction payment.Transaction
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
			id,
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
		FROM payment_transactions
		WHERE processor = $1 AND external_payment_id = $2`,
		processor,
		externalPaymentID,
	).Scan(
		&transaction.ID,
		&transaction.PaymentIntentID,
		&transaction.Processor,
		&transaction.ExternalPaymentID,
		&transaction.SellerAccountID,
		&transaction.Status,
		&transaction.Currency,
		&transaction.AmountCents,
		&transaction.VerifiedOn,
		&transaction.CreatedOn,
		&transaction.UpdatedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, payment.ErrTransactionDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding payment transaction: %w", err)
	}
	return &transaction, nil
}

func (repository *PaymentTransactionRepository) saveWithTx(
	ctx context.Context,
	tx *sql.Tx,
	transaction *payment.Transaction,
) error {
	if tx == nil {
		return fmt.Errorf("saving payment transaction: transaction boundary is required")
	}
	if transaction == nil {
		return fmt.Errorf("saving payment transaction: payment transaction is required")
	}

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
		ON CONFLICT (processor, external_payment_id) DO UPDATE SET
			payment_intent_id = EXCLUDED.payment_intent_id,
			seller_account_id = EXCLUDED.seller_account_id,
			status = EXCLUDED.status,
			currency = EXCLUDED.currency,
			amount_cents = EXCLUDED.amount_cents,
			verified_on = EXCLUDED.verified_on,
			updated_on = EXCLUDED.updated_on
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
	).Scan(&transaction.ID)
	if err != nil {
		return fmt.Errorf("saving payment transaction: %w", err)
	}
	return nil
}

func rollbackPaymentTransactionTx(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back payment transaction: %v", cause, rollbackErr)
	}
	return cause
}
