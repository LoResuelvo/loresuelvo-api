package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/jackc/pgx/v5/pgconn"
)

type PaymentAccountRepository struct {
	db                             *sql.DB
	authorizationAttemptRepository *AuthorizationAttemptRepository
}

func NewPaymentAccountRepository(db *sql.DB, authorizationAttemptRepository *AuthorizationAttemptRepository) *PaymentAccountRepository {
	return &PaymentAccountRepository{
		db:                             db,
		authorizationAttemptRepository: authorizationAttemptRepository,
	}
}

func (repository *PaymentAccountRepository) SaveFromAuthorization(ctx context.Context, attemptID int, account *paymentaccount.PaymentAccount) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning payment account connection transaction: %w", err)
	}

	if err := repository.saveWithTx(ctx, tx, account); err != nil {
		return rollbackPaymentAccountTx(tx, err)
	}
	if err := repository.authorizationAttemptRepository.markConsumedWithTx(
		ctx,
		tx,
		attemptID,
		account.ProviderID(),
		account.PaymentProvider(),
	); err != nil {
		return rollbackPaymentAccountTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing payment account connection transaction: %w", err)
	}
	return nil
}

func (repository *PaymentAccountRepository) saveWithTx(ctx context.Context, tx *sql.Tx, account *paymentaccount.PaymentAccount) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO provider_payment_accounts
		(provider_id, payment_provider, external_account_id, access_token_ciphertext,
		 refresh_token_ciphertext, token_expires_on, can_receive_marketplace_payments)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		account.ProviderID(),
		account.PaymentProvider(),
		account.ExternalAccountID(),
		account.AccessTokenCiphertext(),
		nullableCiphertext(account.RefreshTokenCiphertext()),
		account.TokenExpiresOn(),
		account.CanReceivePayments(),
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) {
			switch postgresError.ConstraintName {
			case "provider_payment_accounts_provider_unique":
				return paymentaccount.ErrAlreadyConnected
			case "provider_payment_accounts_external_unique":
				return paymentaccount.ErrExternalAccountAlreadyLinked
			}
		}
		return fmt.Errorf("saving provider payment account: %w", err)
	}
	return nil
}

func (repository *PaymentAccountRepository) FindByProviderID(ctx context.Context, providerID int, paymentProvider paymentaccount.PaymentProvider) (*paymentaccount.PaymentAccount, error) {
	var externalAccountID string
	var accessTokenCiphertext []byte
	var refreshTokenCiphertext []byte
	var tokenExpiresOn time.Time
	var canReceiveMarketplacePayments bool
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT external_account_id, access_token_ciphertext, refresh_token_ciphertext,
		        token_expires_on, can_receive_marketplace_payments
		FROM provider_payment_accounts
		WHERE provider_id = $1 AND payment_provider = $2`,
		providerID,
		paymentProvider,
	).Scan(
		&externalAccountID,
		&accessTokenCiphertext,
		&refreshTokenCiphertext,
		&tokenExpiresOn,
		&canReceiveMarketplacePayments,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, paymentaccount.ErrConnectionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding provider payment account: %w", err)
	}
	account, err := paymentaccount.NewPaymentAccount(
		providerID,
		paymentProvider,
		externalAccountID,
		accessTokenCiphertext,
		refreshTokenCiphertext,
		tokenExpiresOn,
		canReceiveMarketplacePayments,
	)
	if err != nil {
		return nil, fmt.Errorf("restoring provider payment account: %w", err)
	}
	return account, nil
}

func (repository *PaymentAccountRepository) DeleteAll() error {
	tx, err := repository.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning payment account cleanup transaction: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM provider_payment_accounts`); err != nil {
		return rollbackPaymentAccountTx(tx, fmt.Errorf("deleting provider payment accounts: %w", err))
	}
	if err := repository.authorizationAttemptRepository.deleteAllWithTx(tx); err != nil {
		return rollbackPaymentAccountTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing payment account cleanup transaction: %w", err)
	}
	return nil
}

func nullableCiphertext(ciphertext []byte) any {
	if len(ciphertext) == 0 {
		return nil
	}
	return ciphertext
}

func rollbackPaymentAccountTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback payment account transaction: %v", originalErr, rollbackErr)
	}
	return originalErr
}
