package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/jackc/pgx/v5/pgconn"
)

type ReviewReportRepository struct{ db *sql.DB }

func NewReviewReportRepository(db *sql.DB) *ReviewReportRepository {
	return &ReviewReportRepository{db: db}
}

var _ workorder.ReviewReportRepository = (*ReviewReportRepository)(nil)

func (r *ReviewReportRepository) Save(ctx context.Context, report *workorder.ReviewReport) error {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO review_reports (work_order_id, reporter_id, category, explanation, status, created_on)
 VALUES ($1, $2, $3, $4, $5, $6)
 RETURNING id`,
		report.WorkOrderID(), report.ReporterID(), report.Category(), report.Explanation(), report.Status(), report.CreatedOn(),
	).Scan(&id)
	var constraint *pgconn.PgError
	if errors.As(err, &constraint) && constraint.Code == "23505" && constraint.ConstraintName == "review_reports_work_order_id_key" {
		return workorder.ErrReviewReportAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("inserting review report: %w", err)
	}
	report.SetID(id)
	return nil
}
func (r *ReviewReportRepository) FindByWorkOrderID(ctx context.Context, orderID int) (*workorder.ReviewReport, error) {
	var id, reporterID int
	var category, explanation, status string
	var at time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT id, reporter_id, category, explanation, status, created_on
 FROM review_reports WHERE work_order_id = $1`,
		orderID,
	).Scan(&id, &reporterID, &category, &explanation, &status, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrReviewReportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding review report: %w", err)
	}
	return workorder.RestoreReviewReport(id, orderID, reporterID, category, explanation, status, at)
}
