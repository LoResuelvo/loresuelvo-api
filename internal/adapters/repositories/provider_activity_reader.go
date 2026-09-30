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

// ProviderActivityReader reads persisted activity and live pending work from a
// single, read-only database snapshot.
type ProviderActivityReader struct{ db *sql.DB }

func NewProviderActivityReader(db *sql.DB) *ProviderActivityReader {
	return &ProviderActivityReader{db: db}
}

const providerActivityTotalsSQL = `
WITH first_completion_by_consumer AS (
	SELECT sp.provider_id, sp.consumer_id, MIN(report.reported_on) AS first_reported_on
	FROM work_order_completion_reports report
	JOIN work_orders wo ON wo.id = report.work_order_id
	JOIN service_proposals sp ON sp.id = wo.service_proposal_id
	WHERE sp.provider_id = $1
	GROUP BY sp.provider_id, sp.consumer_id
), activity_orders AS (
	SELECT sp.provider_id, sp.consumer_id, sp.amount_cents,
		wo.accepted_on, wo.paid_on, report.reported_on, first_completion.first_reported_on
	FROM work_orders wo
	JOIN service_proposals sp ON sp.id = wo.service_proposal_id
	LEFT JOIN work_order_completion_reports report ON report.work_order_id = wo.id
	LEFT JOIN first_completion_by_consumer first_completion
		ON first_completion.provider_id = sp.provider_id
		AND first_completion.consumer_id = sp.consumer_id
	WHERE sp.provider_id = $1
	  AND (
		(wo.accepted_on >= $3 AND wo.accepted_on < $4)
		OR (report.reported_on >= $3 AND report.reported_on < $4)
		OR (wo.paid_on >= $3 AND wo.paid_on < $4)
	  )
)
SELECT
	COUNT(*) FILTER (WHERE accepted_on >= $2 AND accepted_on < $4),
	COUNT(*) FILTER (WHERE reported_on >= $2 AND reported_on < $4),
	COUNT(*) FILTER (WHERE paid_on >= $2 AND paid_on < $4),
	COUNT(DISTINCT consumer_id) FILTER (WHERE reported_on >= $2 AND reported_on < $4),
	COUNT(DISTINCT consumer_id) FILTER (WHERE reported_on >= $2 AND reported_on < $4 AND first_reported_on >= $2),
	COUNT(DISTINCT consumer_id) FILTER (WHERE reported_on >= $2 AND reported_on < $4 AND first_reported_on < $2),
	COALESCE(SUM(amount_cents::numeric) FILTER (WHERE reported_on >= $2 AND reported_on < $4), 0)::text,
	COUNT(*) FILTER (WHERE accepted_on >= $3 AND accepted_on < $2),
	COUNT(*) FILTER (WHERE reported_on >= $3 AND reported_on < $2),
	COUNT(*) FILTER (WHERE paid_on >= $3 AND paid_on < $2),
	COUNT(DISTINCT consumer_id) FILTER (WHERE reported_on >= $3 AND reported_on < $2),
	COUNT(DISTINCT consumer_id) FILTER (WHERE reported_on >= $3 AND reported_on < $2 AND first_reported_on >= $3),
	COUNT(DISTINCT consumer_id) FILTER (WHERE reported_on >= $3 AND reported_on < $2 AND first_reported_on < $3),
	COALESCE(SUM(amount_cents::numeric) FILTER (WHERE reported_on >= $3 AND reported_on < $2), 0)::text
FROM activity_orders`

const providerActivityBucketsSQL = `
WITH events AS (
	SELECT wo.accepted_on AS occurred_on, 'confirmed' AS event_type
	FROM work_orders wo JOIN service_proposals sp ON sp.id = wo.service_proposal_id
	WHERE sp.provider_id = $1 AND wo.accepted_on >= $2 AND wo.accepted_on < $3
	UNION ALL
	SELECT report.reported_on, 'reported'
	FROM work_order_completion_reports report
	JOIN work_orders wo ON wo.id = report.work_order_id
	JOIN service_proposals sp ON sp.id = wo.service_proposal_id
	WHERE sp.provider_id = $1 AND report.reported_on >= $2 AND report.reported_on < $3
	UNION ALL
	SELECT wo.paid_on, 'paid'
	FROM work_orders wo JOIN service_proposals sp ON sp.id = wo.service_proposal_id
	WHERE sp.provider_id = $1 AND wo.paid_on >= $2 AND wo.paid_on < $3
)
SELECT date_trunc($4, occurred_on AT TIME ZONE 'UTC' AT TIME ZONE 'America/Argentina/Buenos_Aires')
		AT TIME ZONE 'America/Argentina/Buenos_Aires' AS bucket_from,
	COUNT(*) FILTER (WHERE event_type = 'confirmed'),
	COUNT(*) FILTER (WHERE event_type = 'reported'),
	COUNT(*) FILTER (WHERE event_type = 'paid')
FROM events
GROUP BY bucket_from
ORDER BY bucket_from`

const providerActivityPendingSQL = `
SELECT
	(SELECT COUNT(*) FROM job_requests WHERE provider_id = $1 AND status = 'pending'),
	(SELECT COUNT(*) FROM work_orders wo JOIN service_proposals sp ON sp.id = wo.service_proposal_id WHERE sp.provider_id = $1 AND wo.status = 'scheduled'),
	(SELECT COUNT(*) FROM work_orders wo JOIN service_proposals sp ON sp.id = wo.service_proposal_id WHERE sp.provider_id = $1 AND wo.status = 'awaiting_payment' AND EXISTS (SELECT 1 FROM work_order_completion_reports report WHERE report.work_order_id = wo.id))`

func (r *ProviderActivityReader) Read(ctx context.Context, providerID int, query provider.ActivityQuery) (_ *readmodel.ActivitySnapshot, err error) {
	if providerID <= 0 {
		return nil, fmt.Errorf("reading provider activity: provider ID must be positive")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning provider activity snapshot: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rolling back provider activity snapshot: %w", rollbackErr))
		}
	}()
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM providers p JOIN users u ON u.id = p.user_id
			WHERE p.user_id = $1 AND u.role = 'provider'
		)`, providerID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("checking provider activity actor: %w", err)
	}
	if !exists {
		return nil, provider.ErrActivityProviderNotFound
	}

	snapshot := &readmodel.ActivitySnapshot{Series: make([]readmodel.ActivityBucket, 0)}
	previous := query.PreviousPeriod()
	var previousFrom time.Time
	if query.ComparePrevious {
		previousFrom = previous.From
	} else {
		previousFrom = query.From
	}
	var currentValue, previousValue string
	previousCounts := readmodel.ActivityCounts{}
	err = tx.QueryRowContext(ctx, providerActivityTotalsSQL,
		providerID, activityBound(query.From), activityBound(previousFrom), activityBound(query.To),
	).Scan(
		&snapshot.Current.Confirmed, &snapshot.Current.Reported, &snapshot.Current.Paid,
		&snapshot.Current.Customers, &snapshot.Current.NewCustomers, &snapshot.Current.ReturningCustomers,
		&currentValue,
		&previousCounts.Confirmed, &previousCounts.Reported, &previousCounts.Paid,
		&previousCounts.Customers, &previousCounts.NewCustomers, &previousCounts.ReturningCustomers,
		&previousValue,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregating provider activity: %w", err)
	}
	if snapshot.Current.ContractValueCents, err = parseActivityCents(currentValue); err != nil {
		return nil, fmt.Errorf("aggregating provider activity: %w", err)
	}
	if previousCounts.ContractValueCents, err = parseActivityCents(previousValue); err != nil {
		return nil, fmt.Errorf("aggregating provider activity: %w", err)
	}
	if query.ComparePrevious {
		snapshot.Previous = &previousCounts
	}

	rows, err := tx.QueryContext(ctx, providerActivityBucketsSQL, providerID, activityBound(query.From), activityBound(query.To), string(query.Granularity))
	if err != nil {
		return nil, fmt.Errorf("querying provider activity buckets: %w", err)
	}
	for rows.Next() {
		var bucket readmodel.ActivityBucket
		if err := rows.Scan(&bucket.From, &bucket.Confirmed, &bucket.Reported, &bucket.Paid); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning provider activity bucket: %w", err)
		}
		bucket.From = bucket.From.UTC()
		snapshot.Series = append(snapshot.Series, bucket)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating provider activity buckets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing provider activity buckets: %w", err)
	}
	if err := tx.QueryRowContext(ctx, providerActivityPendingSQL, providerID).Scan(
		&snapshot.Pending.Requests, &snapshot.Pending.Scheduled, &snapshot.Pending.AwaitingPayment,
	); err != nil {
		return nil, fmt.Errorf("querying current provider pending work: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing provider activity snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

func parseActivityCents(value string) (int64, error) {
	cents, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("contract value is outside the supported integer range: %w", err)
	}
	return cents, nil
}

// PostgreSQL timestamps retain microseconds. Ceil bounds before binding so
// their comparisons match the exact half-open interval over stored instants.
func activityBound(value time.Time) time.Time {
	value = value.UTC()
	nanosecondRemainder := value.Nanosecond() % int(time.Microsecond)
	if nanosecondRemainder == 0 {
		return value
	}
	return value.Add(time.Microsecond - time.Duration(nanosecondRemainder))
}

var _ provider.ActivityReader = (*ProviderActivityReader)(nil)
