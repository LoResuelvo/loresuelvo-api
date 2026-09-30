package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

// ProviderCollectionReader never consults the payment gateway or payment account.
// Every response is assembled from one read-only, repeatable-read snapshot.
type ProviderCollectionReader struct{ db *sql.DB }

func NewProviderCollectionReader(db *sql.DB) *ProviderCollectionReader {
	return &ProviderCollectionReader{db: db}
}

// This join is one row per local transaction. The work-order relation is unique
// by proposal, so images, notifications and other one-to-many data cannot fan
// out amounts. Validate approved rows before reporting any aggregate or page.
const collectionFactsCTE = `
WITH collection_facts AS (
	SELECT tx.id, tx.verified_on, tx.currency AS transaction_currency,
		tx.amount_cents AS transaction_amount_cents,
		pi.id AS intent_id, pi.purpose, pi.status AS intent_status,
		pi.currency AS intent_currency, pi.seller_amount_cents,
		pi.platform_fee_cents, pi.total_amount_cents, pi.created_on AS intent_created_on,
		sp.id AS proposal_id, sp.status AS proposal_status, sp.currency AS proposal_currency,
		sp.amount_cents AS proposal_amount_cents, sp.deposit_cents,
		sp.platform_fee_total_cents, sp.platform_fee_due_now_cents,
		wo.id AS work_order_id, wo.status AS work_order_status,
		(
			pi.status = 'paid'
			AND pi.currency = 'ARS' AND tx.currency = 'ARS' AND sp.currency = 'ARS'
			AND tx.currency = pi.currency AND tx.amount_cents = pi.total_amount_cents
			AND pi.total_amount_cents = pi.seller_amount_cents + pi.platform_fee_cents
			AND sp.status = 'accepted' AND wo.id IS NOT NULL
			AND pi.created_on <= tx.verified_on
			AND (
				(pi.purpose = 'booking_deposit'
					AND pi.seller_amount_cents = sp.deposit_cents
					AND pi.platform_fee_cents = sp.platform_fee_due_now_cents)
				OR (pi.purpose = 'service_balance'
					AND pi.seller_amount_cents = sp.amount_cents - sp.deposit_cents
					AND pi.platform_fee_cents = sp.platform_fee_total_cents - sp.platform_fee_due_now_cents
					AND wo.status = 'paid')
			)
			AND NOT EXISTS (
				SELECT 1 FROM payment_transactions duplicate
				WHERE duplicate.payment_intent_id = pi.id
					AND duplicate.status = 'approved' AND duplicate.id <> tx.id
			)
		) AS valid
	FROM payment_transactions tx
	JOIN payment_intents pi ON pi.id = tx.payment_intent_id
	JOIN service_proposals sp ON sp.id = pi.service_proposal_id
	LEFT JOIN work_orders wo ON wo.service_proposal_id = sp.id
	WHERE sp.provider_id = $1 AND tx.status = 'approved'
		AND tx.verified_on >= $2 AND tx.verified_on < $3
)
`

const collectionValidationSQL = collectionFactsCTE + `
SELECT COUNT(*) FILTER (WHERE NOT valid) FROM collection_facts`

const collectionSummaryTotalsSQL = collectionFactsCTE + `
SELECT
	COALESCE(SUM(seller_amount_cents::numeric) FILTER (WHERE verified_on >= $4 AND purpose = 'booking_deposit'), 0)::text,
	COALESCE(SUM(seller_amount_cents::numeric) FILTER (WHERE verified_on >= $4 AND purpose = 'service_balance'), 0)::text,
	COALESCE(SUM(seller_amount_cents::numeric) FILTER (WHERE verified_on < $4 AND purpose = 'booking_deposit'), 0)::text,
	COALESCE(SUM(seller_amount_cents::numeric) FILTER (WHERE verified_on < $4 AND purpose = 'service_balance'), 0)::text
FROM collection_facts`

const collectionBucketsSQL = collectionFactsCTE + `
SELECT date_trunc($4, verified_on AT TIME ZONE 'America/Argentina/Buenos_Aires')
		AT TIME ZONE 'America/Argentina/Buenos_Aires' AS bucket_from,
	COALESCE(SUM(seller_amount_cents::numeric) FILTER (WHERE purpose = 'booking_deposit'), 0)::text,
	COALESCE(SUM(seller_amount_cents::numeric) FILTER (WHERE purpose = 'service_balance'), 0)::text
FROM collection_facts
GROUP BY bucket_from
ORDER BY bucket_from`

const collectionPendingSQL = `
SELECT
	COUNT(*) FILTER (WHERE wo.status = 'scheduled'),
	COALESCE(SUM((sp.amount_cents::numeric - sp.deposit_cents::numeric)) FILTER (WHERE wo.status = 'scheduled'), 0)::text,
	COUNT(*) FILTER (WHERE wo.status = 'awaiting_payment'),
	COALESCE(SUM((sp.amount_cents::numeric - sp.deposit_cents::numeric)) FILTER (WHERE wo.status = 'awaiting_payment'), 0)::text,
	COUNT(*) FILTER (WHERE sp.status <> 'accepted' OR sp.currency <> 'ARS'
		OR (wo.status = 'scheduled' AND report.work_order_id IS NOT NULL)
		OR (wo.status = 'awaiting_payment' AND report.work_order_id IS NULL))
FROM work_orders wo
JOIN service_proposals sp ON sp.id = wo.service_proposal_id
LEFT JOIN work_order_completion_reports report ON report.work_order_id = wo.id
WHERE sp.provider_id = $1 AND wo.status IN ('scheduled', 'awaiting_payment')`

const collectionDetailTotalsSQL = collectionFactsCTE + `
SELECT COUNT(*), COALESCE(SUM(seller_amount_cents::numeric), 0)::text
FROM collection_facts WHERE ($4::text = '' OR purpose = $4)`

const collectionDetailPageSQL = collectionFactsCTE + `
SELECT id, verified_on, purpose, seller_amount_cents, transaction_currency, proposal_id, work_order_id
FROM collection_facts
WHERE ($4::text = '' OR purpose = $4)
	AND ($5::timestamptz IS NULL OR (verified_on, id) < ($5::timestamptz, $6::bigint))
ORDER BY verified_on DESC, id DESC
LIMIT $7`

func (r *ProviderCollectionReader) ReadSummary(ctx context.Context, providerID int, query provider.ActivityQuery) (_ *readmodel.CollectionSnapshot, err error) {
	tx, err := r.begin(ctx, providerID)
	if err != nil {
		return nil, err
	}
	committed := false
	defer rollbackCollectionTx(tx, &err, &committed)

	previousFrom := query.From
	if query.ComparePrevious {
		previousFrom = query.PreviousPeriod().From
	}
	start, from, end := activityBound(previousFrom), activityBound(query.From), activityBound(query.To)
	if err := validateCollectionFacts(ctx, tx, providerID, start, end); err != nil {
		return nil, err
	}
	snapshot := &readmodel.CollectionSnapshot{Series: make([]readmodel.CollectionBucket, 0)}
	var currentDeposit, currentBalance, previousDeposit, previousBalance string
	if err := tx.QueryRowContext(ctx, collectionSummaryTotalsSQL, providerID, start, end, from).Scan(
		&currentDeposit, &currentBalance, &previousDeposit, &previousBalance,
	); err != nil {
		return nil, fmt.Errorf("aggregating provider collections: %w", err)
	}
	if snapshot.Current.BookingDepositCents, err = parseCollectionCents(currentDeposit); err != nil {
		return nil, err
	}
	if snapshot.Current.ServiceBalanceCents, err = parseCollectionCents(currentBalance); err != nil {
		return nil, err
	}
	if query.ComparePrevious {
		previous := &readmodel.CollectionAmounts{}
		if previous.BookingDepositCents, err = parseCollectionCents(previousDeposit); err != nil {
			return nil, err
		}
		if previous.ServiceBalanceCents, err = parseCollectionCents(previousBalance); err != nil {
			return nil, err
		}
		snapshot.Previous = previous
	}
	rows, err := tx.QueryContext(ctx, collectionBucketsSQL, providerID, from, end, string(query.Granularity))
	if err != nil {
		return nil, fmt.Errorf("querying collection buckets: %w", err)
	}
	for rows.Next() {
		var bucket readmodel.CollectionBucket
		var deposit, balance string
		if err := rows.Scan(&bucket.From, &deposit, &balance); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning collection bucket: %w", err)
		}
		bucket.From = bucket.From.UTC()
		if bucket.BookingDepositCents, err = parseCollectionCents(deposit); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if bucket.ServiceBalanceCents, err = parseCollectionCents(balance); err != nil {
			_ = rows.Close()
			return nil, err
		}
		snapshot.Series = append(snapshot.Series, bucket)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating collection buckets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing collection buckets: %w", err)
	}
	var scheduledAmount, awaitingAmount string
	var invalidPending int64
	if err := tx.QueryRowContext(ctx, collectionPendingSQL, providerID).Scan(
		&snapshot.Pending.Scheduled.Orders, &scheduledAmount,
		&snapshot.Pending.AwaitingPayment.Orders, &awaitingAmount, &invalidPending,
	); err != nil {
		return nil, fmt.Errorf("reading current collection pending balances: %w", err)
	}
	if invalidPending != 0 {
		return nil, errors.New("collection pending balances are inconsistent")
	}
	if snapshot.Pending.Scheduled.AmountCents, err = parseCollectionCents(scheduledAmount); err != nil {
		return nil, err
	}
	if snapshot.Pending.AwaitingPayment.AmountCents, err = parseCollectionCents(awaitingAmount); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing provider collections snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

func (r *ProviderCollectionReader) ReadDetail(ctx context.Context, providerID int, query provider.CollectionDetailQuery) (_ *readmodel.CollectionDetailSnapshot, err error) {
	tx, err := r.begin(ctx, providerID)
	if err != nil {
		return nil, err
	}
	committed := false
	defer rollbackCollectionTx(tx, &err, &committed)
	start, end := activityBound(query.Period.From), activityBound(query.Period.To)
	if err := validateCollectionFacts(ctx, tx, providerID, start, end); err != nil {
		return nil, err
	}
	snapshot := &readmodel.CollectionDetailSnapshot{Page: make([]readmodel.CollectionTransaction, 0)}
	var amount string
	if err := tx.QueryRowContext(ctx, collectionDetailTotalsSQL, providerID, start, end, string(query.Purpose)).Scan(&snapshot.TotalCount, &amount); err != nil {
		return nil, fmt.Errorf("aggregating collection detail: %w", err)
	}
	if snapshot.TotalAmountCents, err = parseCollectionCents(amount); err != nil {
		return nil, err
	}
	var afterTime any
	var afterID any
	if query.After != nil {
		afterTime, afterID = query.After.VerifiedOn.UTC(), query.After.ID
	}
	rows, err := tx.QueryContext(ctx, collectionDetailPageSQL, providerID, start, end, string(query.Purpose), afterTime, afterID, query.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("querying collection detail page: %w", err)
	}
	for rows.Next() {
		var transaction readmodel.CollectionTransaction
		var workOrderID sql.NullInt64
		if err := rows.Scan(&transaction.ID, &transaction.VerifiedOn, &transaction.Purpose, &transaction.SellerAmountCents, &transaction.Currency, &transaction.ServiceProposalID, &workOrderID); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning collection detail row: %w", err)
		}
		transaction.VerifiedOn = transaction.VerifiedOn.UTC()
		if workOrderID.Valid {
			id := int(workOrderID.Int64)
			transaction.WorkOrderID = &id
		}
		snapshot.Page = append(snapshot.Page, transaction)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating collection detail page: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing collection detail page: %w", err)
	}
	if len(snapshot.Page) > query.Limit {
		snapshot.Page = snapshot.Page[:query.Limit]
		last := snapshot.Page[len(snapshot.Page)-1]
		snapshot.Next = &readmodel.CollectionPosition{VerifiedOn: last.VerifiedOn, ID: last.ID}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing provider collection detail snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

func (r *ProviderCollectionReader) begin(ctx context.Context, providerID int) (*sql.Tx, error) {
	if providerID <= 0 {
		return nil, errors.New("collection provider ID must be positive")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning collection snapshot: %w", err)
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM providers p JOIN users u ON u.id = p.user_id
			WHERE p.user_id = $1 AND u.role = 'provider'
		)`, providerID).Scan(&exists); err != nil {
		return nil, rollbackCollectionBegin(tx, fmt.Errorf("checking collection provider: %w", err))
	}
	if !exists {
		return nil, rollbackCollectionBegin(tx, provider.ErrActivityProviderNotFound)
	}
	return tx, nil
}

func rollbackCollectionBegin(tx *sql.Tx, cause error) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return errors.Join(cause, fmt.Errorf("rolling back collection snapshot: %w", err))
	}
	return cause
}

func validateCollectionFacts(ctx context.Context, tx *sql.Tx, providerID int, start, end time.Time) error {
	var invalid int64
	if err := tx.QueryRowContext(ctx, collectionValidationSQL, providerID, start, end).Scan(&invalid); err != nil {
		return fmt.Errorf("validating collection transactions: %w", err)
	}
	if invalid != 0 {
		return errors.New("approved collection transactions are inconsistent")
	}
	return nil
}

func parseCollectionCents(value string) (int64, error) {
	cents, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("collection amount is outside the supported integer range: %w", err)
	}
	if cents < 0 {
		return 0, errors.New("collection amount is negative")
	}
	return cents, nil
}

func rollbackCollectionTx(tx *sql.Tx, resultErr *error, committed *bool) {
	if *committed {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		*resultErr = errors.Join(*resultErr, fmt.Errorf("rolling back collection snapshot: %w", err))
	}
}

var _ provider.CollectionReader = (*ProviderCollectionReader)(nil)
