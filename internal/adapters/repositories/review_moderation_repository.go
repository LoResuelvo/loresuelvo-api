package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type reviewExecutor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}
type ReviewModerationRepository struct{}

func NewReviewModerationRepository() *ReviewModerationRepository {
	return &ReviewModerationRepository{}
}
func (r *ReviewModerationRepository) findWithExecutor(ctx context.Context, e reviewExecutor, id int) (*workorder.Review, error) {
	var rating, version int
	var description string
	var visible bool
	var hiding sql.NullInt64
	err := e.QueryRowContext(ctx, `SELECT rating,description,visible,moderation_version,hiding_decision_id FROM work_order_reviews WHERE work_order_id=$1`, id).Scan(&rating, &description, &visible, &version, &hiding)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrReviewNotAvailable
	}
	if err != nil {
		return nil, fmt.Errorf("finding review: %w", err)
	}
	return workorder.RestoreReview(id, workorder.ReviewRestoreInput{Rating: rating, Description: description, Visible: &visible, Version: version, HidingDecisionID: int(hiding.Int64)})
}
func (r *ReviewModerationRepository) saveWithExecutor(ctx context.Context, e reviewExecutor, id int, review *workorder.Review) error {
	var hiding any
	if review.HidingDecisionID() > 0 {
		hiding = review.HidingDecisionID()
	}
	result, err := e.ExecContext(ctx, `UPDATE work_order_reviews SET visible=$2,moderation_version=$3,hiding_decision_id=$4 WHERE work_order_id=$1`, id, review.Visible(), review.Version(), hiding)
	if err != nil {
		return fmt.Errorf("saving review moderation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return workorder.ErrReviewNotAvailable
	}
	return nil
}
func (r *ReviewModerationRepository) saveDecisionWithExecutor(ctx context.Context, e reviewExecutor, d *workorder.ReviewDecision) error {
	var category, report, previous any
	if d.Category() != "" {
		category = d.Category()
	}
	if d.ReportID() > 0 {
		report = d.ReportID()
	}
	if d.PreviousHideID() > 0 {
		previous = d.PreviousHideID()
	}
	var id int
	err := e.QueryRowContext(ctx, `INSERT INTO review_decisions(work_order_id,operator_id,action,category,reason,report_id,previous_hide_id,created_on) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, d.WorkOrderID(), d.OperatorID(), d.Action(), category, d.Reason(), report, previous, d.CreatedOn()).Scan(&id)
	if err != nil {
		return fmt.Errorf("saving review decision: %w", err)
	}
	d.SetID(id)
	return nil
}
func scanReviewDecision(row interface{ Scan(...any) error }) (*workorder.ReviewDecision, error) {
	var id, order, operator int
	var report, previous sql.NullInt64
	var action, reason string
	var category sql.NullString
	var at time.Time
	if err := row.Scan(&id, &order, &operator, &action, &category, &reason, &report, &previous, &at); err != nil {
		return nil, err
	}
	return workorder.RestoreReviewDecision(id, order, operator, int(report.Int64), int(previous.Int64), action, category.String, reason, at)
}
