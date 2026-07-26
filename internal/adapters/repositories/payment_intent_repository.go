package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
)

const mercadoPagoProcessor = "mercado_pago"

type PaymentIntentRepository struct {
	db *sql.DB
}

func NewPaymentIntentRepository(db *sql.DB) *PaymentIntentRepository {
	return &PaymentIntentRepository{db: db}
}

func (repository *PaymentIntentRepository) Save(ctx context.Context, intent *payment.Intent) error {
	if intent == nil {
		return fmt.Errorf("saving payment intent: payment intent is required")
	}
	_, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO payment_intents (
			id,
			service_proposal_id,
			purpose,
			currency,
			seller_amount_cents,
			platform_fee_cents,
			total_amount_cents,
			status,
			created_on,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		intent.ID,
		intent.ServiceProposalID,
		intent.Purpose,
		intent.Currency,
		intent.SellerAmountCents,
		intent.PlatformFeeCents,
		intent.TotalAmountCents,
		intent.Status,
		intent.CreatedOn,
		intent.UpdatedOn,
	)
	if err != nil {
		return fmt.Errorf("saving payment intent: %w", err)
	}
	return nil
}

func (repository *PaymentIntentRepository) SaveCheckoutReady(ctx context.Context, intent *payment.Intent) error {
	if intent == nil || intent.CheckoutSession == nil || intent.Status != payment.StatusCheckoutReady {
		return fmt.Errorf("saving checkout-ready payment intent: ready intent and checkout session are required")
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning checkout-ready payment intent transaction: %w", err)
	}
	result, err := tx.ExecContext(
		ctx,
		`UPDATE payment_intents
		SET status = $1, updated_on = $2
		WHERE id = $3 AND status = $4`,
		intent.Status,
		intent.UpdatedOn,
		intent.ID,
		payment.StatusRequiresCheckout,
	)
	if err != nil {
		return rollbackPaymentIntentTx(tx, fmt.Errorf("updating checkout-ready payment intent: %w", err))
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return rollbackPaymentIntentTx(tx, fmt.Errorf("checking checkout-ready payment intent update: %w", err))
	}
	if affected != 1 {
		return rollbackPaymentIntentTx(tx, fmt.Errorf("updating checkout-ready payment intent: unexpected current status"))
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO payment_checkout_sessions (
			payment_intent_id,
			processor,
			external_preference_id,
			checkout_url,
			expires_on,
			created_on
		)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		intent.ID,
		mercadoPagoProcessor,
		intent.CheckoutSession.ExternalID,
		intent.CheckoutSession.URL,
		intent.CheckoutSession.ExpiresOn,
		intent.CheckoutSession.CreatedOn,
	)
	if err != nil {
		return rollbackPaymentIntentTx(tx, fmt.Errorf("saving payment checkout session: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing checkout-ready payment intent transaction: %w", err)
	}
	return nil
}

func (repository *PaymentIntentRepository) FindByID(ctx context.Context, id string) (*payment.Intent, error) {
	var intent payment.Intent
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			service_proposal_id,
			purpose,
			currency,
			seller_amount_cents,
			platform_fee_cents,
			total_amount_cents,
			status,
			created_on,
			updated_on
		FROM payment_intents
		WHERE id = $1`,
		id,
	).Scan(
		&intent.ID,
		&intent.ServiceProposalID,
		&intent.Purpose,
		&intent.Currency,
		&intent.SellerAmountCents,
		&intent.PlatformFeeCents,
		&intent.TotalAmountCents,
		&intent.Status,
		&intent.CreatedOn,
		&intent.UpdatedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, payment.ErrIntentDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding payment intent: %w", err)
	}
	return &intent, nil
}

func (repository *PaymentIntentRepository) SaveProcessing(ctx context.Context, intent *payment.Intent) error {
	if intent == nil || intent.Status != payment.StatusProcessing {
		return fmt.Errorf("saving processing payment intent: processing intent is required")
	}
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE payment_intents
		SET status = $1, updated_on = $2
		WHERE id = $3 AND status IN ($4, $5)`,
		intent.Status,
		intent.UpdatedOn,
		intent.ID,
		payment.StatusCheckoutReady,
		payment.StatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("saving processing payment intent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking processing payment intent update: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("saving processing payment intent: unexpected current status")
	}
	return nil
}

func (repository *PaymentIntentRepository) markPaidWithTx(
	ctx context.Context,
	tx *sql.Tx,
	intent *payment.Intent,
) error {
	if tx == nil {
		return fmt.Errorf("marking payment intent paid: transaction is required")
	}
	if intent == nil || intent.Status != payment.StatusPaid {
		return fmt.Errorf("marking payment intent paid: paid intent is required")
	}
	result, err := tx.ExecContext(
		ctx,
		`UPDATE payment_intents
		SET status = $1, updated_on = $2
		WHERE id = $3 AND status IN ($4, $5)`,
		intent.Status,
		intent.UpdatedOn,
		intent.ID,
		payment.StatusCheckoutReady,
		payment.StatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("marking payment intent paid: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking paid payment intent update: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("marking payment intent paid: unexpected current status")
	}
	return nil
}

func (repository *PaymentIntentRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM payment_intents`)
	if err != nil {
		return fmt.Errorf("deleting payment intents: %w", err)
	}
	return nil
}

func rollbackPaymentIntentTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback payment intent transaction: %v", originalErr, rollbackErr)
	}
	return originalErr
}
