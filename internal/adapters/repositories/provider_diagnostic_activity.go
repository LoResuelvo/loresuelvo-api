package repositories

import (
	"context"
	"database/sql"
	"fmt"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
)

const diagnosticActivitySQL = `SELECT type,id,status,occurred_on FROM (
 SELECT 'job_request' AS type,id,status,created_on AS occurred_on FROM job_requests WHERE provider_id=$1
 UNION ALL SELECT 'service_proposal',id,status,created_on FROM service_proposals WHERE provider_id=$1
 UNION ALL SELECT 'work_order',w.id,w.status,w.accepted_on FROM work_orders w JOIN service_proposals p ON p.id=w.service_proposal_id WHERE p.provider_id=$1
 ) activity ORDER BY occurred_on DESC,type DESC,id DESC LIMIT $2`
const diagnosticReviewsSQL = `SELECT r.work_order_id,r.rating,r.description FROM work_order_reviews r JOIN work_orders w ON w.id=r.work_order_id JOIN service_proposals p ON p.id=w.service_proposal_id WHERE p.provider_id=$1 AND r.visible ORDER BY w.accepted_on DESC,w.id DESC LIMIT $2`
const diagnosticRatingSQL = `SELECT COALESCE(SUM(r.rating),0),COUNT(*) FROM work_order_reviews r JOIN work_orders w ON w.id=r.work_order_id JOIN service_proposals p ON p.id=w.service_proposal_id WHERE p.provider_id=$1`
const diagnosticOrderSyncSQL = `SELECT w.id,e.synced_on FROM work_orders w JOIN service_proposals p ON p.id=w.service_proposal_id LEFT JOIN work_order_calendar_events e ON e.work_order_id=w.id AND e.user_id=p.provider_id WHERE p.provider_id=$1 ORDER BY w.accepted_on DESC,w.id DESC LIMIT $2`

func readDiagnosticActivity(ctx context.Context, tx *sql.Tx, id int, d *readmodel.ProviderDiagnostic) error {
	var err error
	d.Activity, err = readDiagnosticReferences(ctx, tx, id)
	if err != nil {
		return err
	}
	d.Reviews, err = readDiagnosticReviews(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, diagnosticRatingSQL, id).Scan(&d.RatingStats.Total, &d.RatingStats.Count); err != nil {
		return fmt.Errorf("reading diagnostic reputation: %w", err)
	}
	d.OrderSync, err = readDiagnosticOrderSync(ctx, tx, id)
	return err
}
func readDiagnosticReferences(ctx context.Context, tx *sql.Tx, id int) ([]readmodel.ProviderActivity, error) {
	rows, err := tx.QueryContext(ctx, diagnosticActivitySQL, id, readmodel.DiagnosticReferenceLimit)
	if err != nil {
		return nil, fmt.Errorf("reading diagnostic activity: %w", err)
	}
	defer rows.Close()
	activity := make([]readmodel.ProviderActivity, 0)
	for rows.Next() {
		var a readmodel.ProviderActivity
		if err := rows.Scan(&a.Type, &a.ID, &a.Status, &a.OccurredOn); err != nil {
			return nil, fmt.Errorf("scanning diagnostic activity: %w", err)
		}
		a.OccurredOn = a.OccurredOn.UTC()
		activity = append(activity, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating diagnostic activity: %w", err)
	}
	return activity, nil
}
func readDiagnosticReviews(ctx context.Context, tx *sql.Tx, id int) ([]readmodel.ProviderReview, error) {
	rows, err := tx.QueryContext(ctx, diagnosticReviewsSQL, id, readmodel.DiagnosticReferenceLimit)
	if err != nil {
		return nil, fmt.Errorf("reading diagnostic reviews: %w", err)
	}
	defer rows.Close()
	reviews := make([]readmodel.ProviderReview, 0)
	for rows.Next() {
		var r readmodel.ProviderReview
		if err := rows.Scan(&r.WorkOrderID, &r.Rating, &r.Description); err != nil {
			return nil, fmt.Errorf("scanning diagnostic reviews: %w", err)
		}
		reviews = append(reviews, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating diagnostic reviews: %w", err)
	}
	return reviews, nil
}
func readDiagnosticOrderSync(ctx context.Context, tx *sql.Tx, id int) ([]readmodel.ProviderOrderSync, error) {
	rows, err := tx.QueryContext(ctx, diagnosticOrderSyncSQL, id, readmodel.DiagnosticReferenceLimit)
	if err != nil {
		return nil, fmt.Errorf("reading diagnostic order synchronization: %w", err)
	}
	defer rows.Close()
	evidence := make([]readmodel.ProviderOrderSync, 0)
	for rows.Next() {
		var e readmodel.ProviderOrderSync
		var synced sql.NullTime
		if err := rows.Scan(&e.WorkOrderID, &synced); err != nil {
			return nil, fmt.Errorf("scanning diagnostic order synchronization: %w", err)
		}
		e.SyncedOn = diagnosticTime(synced)
		evidence = append(evidence, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating diagnostic order synchronization: %w", err)
	}
	return evidence, nil
}
