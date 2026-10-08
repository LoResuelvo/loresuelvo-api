package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

// ProviderReputationReader reads global paid-order facts and one review page
// from a single read-only, repeatable-read database snapshot.
type ProviderReputationReader struct{ db *sql.DB }

func NewProviderReputationReader(db *sql.DB) *ProviderReputationReader {
	return &ProviderReputationReader{db: db}
}

// Reviews have work_order_id as their primary key. No images, payment attempts
// or other one-to-many relation participates in either reputation query.
const providerReputationTotalsSQL = `
SELECT COUNT(*), COUNT(review.work_order_id),
	COUNT(review.work_order_id) FILTER (WHERE review.visible),
	COUNT(*) FILTER (WHERE review.rating = 1),
	COUNT(*) FILTER (WHERE review.rating = 2),
	COUNT(*) FILTER (WHERE review.rating = 3),
	COUNT(*) FILTER (WHERE review.rating = 4),
	COUNT(*) FILTER (WHERE review.rating = 5)
FROM work_orders wo
JOIN service_proposals sp ON sp.id = wo.service_proposal_id
LEFT JOIN work_order_reviews review ON review.work_order_id = wo.id
WHERE sp.provider_id = $1 AND wo.status = 'paid'`

const providerReputationPageSQL = `
SELECT wo.id, review.rating, review.description
FROM work_orders wo
JOIN service_proposals sp ON sp.id = wo.service_proposal_id
JOIN work_order_reviews review ON review.work_order_id = wo.id
WHERE sp.provider_id = $1 AND wo.status = 'paid' AND review.visible AND ($2 = 0 OR wo.id < $2)
ORDER BY wo.id DESC
LIMIT $3`

func (r *ProviderReputationReader) Read(ctx context.Context, providerID int, query provider.ReputationQuery) (_ *readmodel.ReputationSnapshot, err error) {
	if providerID <= 0 || query.Limit < 1 || query.Limit > provider.MaxReputationPageSize || (query.After != nil && query.After.WorkOrderID <= 0) {
		return nil, provider.ErrInvalidReputationQuery
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning provider reputation snapshot: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rolling back provider reputation snapshot: %w", rollbackErr))
		}
	}()

	// Establish the snapshot before reading any paid-order totals or page.
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM providers p JOIN users u ON u.id = p.user_id
			WHERE p.user_id = $1 AND u.role = 'provider'
		)`, providerID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("checking reputation provider: %w", err)
	}
	if !exists {
		return nil, provider.ErrReputationProviderNotFound
	}

	snapshot := &readmodel.ReputationSnapshot{Reviews: make([]readmodel.ReputationReview, 0)}
	if err := tx.QueryRowContext(ctx, providerReputationTotalsSQL, providerID).Scan(
		&snapshot.EligiblePaidOrders, &snapshot.ReviewedPaidOrders, &snapshot.VisibleReviews,
		&snapshot.RatingDistribution[0], &snapshot.RatingDistribution[1],
		&snapshot.RatingDistribution[2], &snapshot.RatingDistribution[3],
		&snapshot.RatingDistribution[4],
	); err != nil {
		return nil, fmt.Errorf("aggregating provider reputation: %w", err)
	}
	afterID := 0
	if query.After != nil {
		afterID = query.After.WorkOrderID
	}
	rows, err := tx.QueryContext(ctx, providerReputationPageSQL, providerID, afterID, query.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("querying provider reputation reviews: %w", err)
	}
	for rows.Next() {
		var review readmodel.ReputationReview
		if err := rows.Scan(&review.WorkOrderID, &review.Rating, &review.Description); err != nil {
			return nil, errors.Join(fmt.Errorf("scanning provider reputation review: %w", err), rows.Close())
		}
		snapshot.Reviews = append(snapshot.Reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("iterating provider reputation reviews: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing provider reputation reviews: %w", err)
	}
	if len(snapshot.Reviews) > query.Limit {
		snapshot.Reviews = snapshot.Reviews[:query.Limit]
		snapshot.Next = &readmodel.ReputationPosition{WorkOrderID: snapshot.Reviews[len(snapshot.Reviews)-1].WorkOrderID}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing provider reputation snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

var _ provider.ReputationReader = (*ProviderReputationReader)(nil)
