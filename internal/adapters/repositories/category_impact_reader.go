package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
)

type CategoryImpactReader struct{ db *sql.DB }

func NewCategoryImpactReader(db *sql.DB) *CategoryImpactReader { return &CategoryImpactReader{db: db} }
func (reader *CategoryImpactReader) FindByCategoryID(ctx context.Context, id int, observedAt time.Time) (*category.Impact, error) {
	return findCategoryImpactWithExecutor(ctx, reader.db, id, observedAt)
}

// One statement observes the category and each count at the same PostgreSQL snapshot.
// Independent scalar aggregates count identities without multiplying one-to-many joins.
func findCategoryImpactWithExecutor(ctx context.Context, executor categoryQueryExecutor, id int, observedAt time.Time) (*category.Impact, error) {
	impact := &category.Impact{ObservedAt: observedAt.UTC()}
	err := executor.QueryRowContext(ctx, `
 SELECT c.id,c.name,c.normalized_name,c.enabled,c.version,
 (SELECT count(*) FROM providers p WHERE p.category_id=c.id),
 (SELECT count(*) FROM job_requests r JOIN providers p ON p.user_id=r.provider_id WHERE p.category_id=c.id AND r.status='pending'),
 (SELECT count(*) FROM job_requests r JOIN providers p ON p.user_id=r.provider_id WHERE p.category_id=c.id AND r.status='accepted'
 AND NOT EXISTS (SELECT 1 FROM service_proposals sp WHERE sp.conversation_id=r.conversation_id)),
 (SELECT count(*) FROM service_proposals sp JOIN providers p ON p.user_id=sp.provider_id WHERE p.category_id=c.id AND sp.status='pending' AND sp.scheduled_on>$2),
 (SELECT count(*) FROM work_orders wo JOIN service_proposals sp ON sp.id=wo.service_proposal_id JOIN providers p ON p.user_id=sp.provider_id WHERE p.category_id=c.id AND wo.status='scheduled'),
 (SELECT count(*) FROM work_orders wo JOIN service_proposals sp ON sp.id=wo.service_proposal_id JOIN providers p ON p.user_id=sp.provider_id WHERE p.category_id=c.id AND wo.status='awaiting_payment')
 FROM categories c WHERE c.id=$1`, id, impact.ObservedAt).Scan(&impact.Category.ID, &impact.Category.Name, &impact.Category.NormalizedName, &impact.Category.Enabled, &impact.Category.Version,
		&impact.Counts.AssignedProviders, &impact.Counts.PendingRequests, &impact.Counts.AcceptedRequestsWithoutProposal, &impact.Counts.PendingProposals, &impact.Counts.ScheduledOrders, &impact.Counts.AwaitingPaymentOrders)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, category.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("reading category impact: %w", err)
	}
	return impact, nil
}
