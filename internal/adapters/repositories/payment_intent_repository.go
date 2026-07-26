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
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning payment intent transaction: %w", err)
	}
	if err := repository.saveWithTx(ctx, tx, intent); err != nil {
		return rollbackPaymentIntentTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing payment intent transaction: %w", err)
	}
	return nil
}

func (repository *PaymentIntentRepository) saveWithTx(
	ctx context.Context,
	tx *sql.Tx,
	intent *payment.Intent,
) error {
	if tx == nil {
		return fmt.Errorf("saving payment intent: transaction is required")
	}
	if intent == nil {
		return fmt.Errorf("saving payment intent: payment intent is required")
	}
	_, err := tx.ExecContext(
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			service_proposal_id = EXCLUDED.service_proposal_id,
			purpose = EXCLUDED.purpose,
			currency = EXCLUDED.currency,
			seller_amount_cents = EXCLUDED.seller_amount_cents,
			platform_fee_cents = EXCLUDED.platform_fee_cents,
			total_amount_cents = EXCLUDED.total_amount_cents,
			status = EXCLUDED.status,
			updated_on = EXCLUDED.updated_on`,
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
	if intent.CheckoutSession == nil {
		return nil
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
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (processor, external_preference_id) DO UPDATE SET
			checkout_url = EXCLUDED.checkout_url,
			expires_on = EXCLUDED.expires_on`,
		intent.ID,
		mercadoPagoProcessor,
		intent.CheckoutSession.ExternalID,
		intent.CheckoutSession.URL,
		intent.CheckoutSession.ExpiresOn,
		intent.CheckoutSession.CreatedOn,
	)
	if err != nil {
		return fmt.Errorf("saving payment checkout session: %w", err)
	}
	return nil
}

func (repository *PaymentIntentRepository) FindByID(ctx context.Context, id string) (*payment.Intent, error) {
	return repository.findOne(
		ctx,
		`WHERE pi.id = $1`,
		id,
	)
}

func (repository *PaymentIntentRepository) FindLatestByProposalIDAndPurpose(
	ctx context.Context,
	serviceProposalID int,
	purpose payment.Purpose,
) (*payment.Intent, error) {
	return repository.findOne(
		ctx,
		`WHERE pi.service_proposal_id = $1 AND pi.purpose = $2
		ORDER BY pi.created_on DESC
		LIMIT 1`,
		serviceProposalID,
		purpose,
	)
}

func (repository *PaymentIntentRepository) findOne(
	ctx context.Context,
	clause string,
	arguments ...any,
) (*payment.Intent, error) {
	var intent payment.Intent
	var externalID, checkoutURL sql.NullString
	var expiresOn, checkoutCreatedOn sql.NullTime
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
			pi.id,
			pi.service_proposal_id,
			pi.purpose,
			pi.currency,
			pi.seller_amount_cents,
			pi.platform_fee_cents,
			pi.total_amount_cents,
			pi.status,
			pi.created_on,
			pi.updated_on,
			pcs.external_preference_id,
			pcs.checkout_url,
			pcs.expires_on,
			pcs.created_on
		FROM payment_intents pi
		LEFT JOIN payment_checkout_sessions pcs ON pcs.payment_intent_id = pi.id
		`+clause,
		arguments...,
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
		&externalID,
		&checkoutURL,
		&expiresOn,
		&checkoutCreatedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, payment.ErrIntentDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding payment intent: %w", err)
	}
	if externalID.Valid && checkoutURL.Valid && expiresOn.Valid && checkoutCreatedOn.Valid {
		intent.CheckoutSession = &payment.CheckoutSession{
			ExternalID: externalID.String,
			URL:        checkoutURL.String,
			ExpiresOn:  expiresOn.Time,
			CreatedOn:  checkoutCreatedOn.Time,
		}
	}
	return &intent, nil
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
