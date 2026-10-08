package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type AdminReviewReader struct {
	db      *sql.DB
	reviews *ReviewModerationRepository
	reports *ReviewReportRepository
}

func NewAdminReviewReader(db *sql.DB, reviews *ReviewModerationRepository, reports *ReviewReportRepository) *AdminReviewReader {
	return &AdminReviewReader{db: db, reviews: reviews, reports: reports}
}

const adminReviewSelection = operationRootsSQL + ` selected_reviews AS (
 SELECT rv.work_order_id,sp.consumer_id,sp.provider_id,o.kind || '-' || o.resource_id AS operation_id,rv.rating,rv.visible,rv.moderation_version,
 CASE WHEN rr.status='pending' THEN 1 ELSE 0 END AS pending_reports,rr.created_on AS reported_on
 FROM work_order_reviews rv JOIN work_orders wo ON wo.id=rv.work_order_id
 JOIN service_proposals sp ON sp.id=wo.service_proposal_id
 JOIN operations o ON o.service_proposal_id=sp.id
 LEFT JOIN review_reports rr ON rr.work_order_id=rv.work_order_id
)`
const adminReviewColumns = `work_order_id,consumer_id,provider_id,operation_id,rating,visible,moderation_version,pending_reports,reported_on`

func scanAdminReviewSummary(row interface{ Scan(...any) error }) (readmodel.AdminReviewSummary, error) {
	var item readmodel.AdminReviewSummary
	var date sql.NullTime
	err := row.Scan(&item.WorkOrderID, &item.ConsumerID, &item.ProviderID, &item.OperationID, &item.Rating, &item.Visible, &item.Version, &item.PendingReports, &date)
	if date.Valid {
		at := date.Time.UTC()
		item.ReportedOn = &at
	}
	return item, err
}
func (r *AdminReviewReader) FindPage(ctx context.Context, input workorder.ReviewListInput) (*workorder.AdminReviewPage, error) {
	input, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	page := &workorder.AdminReviewPage{Items: []readmodel.AdminReviewSummary{}, Page: input.Page, Limit: input.Limit}
	filter := ` WHERE ($1='all' OR ($1='reported' AND pending_reports=1) OR ($1='visible' AND visible) OR ($1='hidden' AND NOT visible))`
	if err = tx.QueryRowContext(ctx, adminReviewSelection+` SELECT count(*) FROM selected_reviews`+filter, input.Status).Scan(&page.Total); err != nil {
		return nil, fmt.Errorf("counting administrative reviews: %w", err)
	}
	rows, err := tx.QueryContext(ctx, adminReviewSelection+` SELECT `+adminReviewColumns+` FROM selected_reviews`+filter+` ORDER BY work_order_id DESC LIMIT $2 OFFSET $3`, input.Status, input.Limit, (input.Page-1)*input.Limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		item, err := scanAdminReviewSummary(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		page.Items = append(page.Items, item)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return page, nil
}
func (r *AdminReviewReader) FindByID(ctx context.Context, id int, page workorder.ReviewPageInput) (*workorder.AdminReviewDetail, error) {
	page, err := page.Normalize()
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	summary, err := scanAdminReviewSummary(tx.QueryRowContext(ctx, adminReviewSelection+` SELECT `+adminReviewColumns+` FROM selected_reviews WHERE work_order_id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrReviewNotAvailable
	}
	if err != nil {
		return nil, err
	}
	review, err := r.reviews.findWithExecutor(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	report, err := r.reports.findWithExecutor(ctx, tx, id)
	if errors.Is(err, workorder.ErrReviewReportNotFound) {
		report = nil
	} else if err != nil {
		return nil, err
	}
	detail := &workorder.AdminReviewDetail{Summary: summary, Review: review, Report: report, Decisions: []*workorder.ReviewDecision{}, Page: page.Page, Limit: page.Limit}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM review_decisions WHERE work_order_id=$1`, id).Scan(&detail.Total); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,work_order_id,operator_id,action,category,reason,report_id,previous_hide_id,created_on FROM review_decisions WHERE work_order_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, id, page.Limit, (page.Page-1)*page.Limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		decision, err := scanReviewDecision(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		detail.Decisions = append(detail.Decisions, decision)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return detail, nil
}
