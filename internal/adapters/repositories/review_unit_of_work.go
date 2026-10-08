package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type ReviewUnitOfWork struct {
	db      *sql.DB
	reviews *ReviewModerationRepository
	reports *ReviewReportRepository
	events  *AuditEventRepository
}

func NewReviewUnitOfWork(db *sql.DB, reviews *ReviewModerationRepository, reports *ReviewReportRepository, events *AuditEventRepository) *ReviewUnitOfWork {
	return &ReviewUnitOfWork{db: db, reviews: reviews, reports: reports, events: events}
}
func (u *ReviewUnitOfWork) Execute(ctx context.Context, operation func(workorder.ReviewStore) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning review transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	store := &reviewTransactionalStore{tx: tx, unit: u}
	if err = operation(store); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("%w: rolling back review transaction: %v", err, rollbackErr)
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing review transaction: %w", err)
	}
	return nil
}

type reviewTransactionalStore struct {
	tx   *sql.Tx
	unit *ReviewUnitOfWork
}

func (s *reviewTransactionalStore) FindReview(ctx context.Context, id int) (*workorder.Review, error) {
	var lockedID int
	err := s.tx.QueryRowContext(ctx, `SELECT work_order_id FROM work_order_reviews WHERE work_order_id=$1 FOR UPDATE`, id).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrReviewNotAvailable
	}
	if err != nil {
		return nil, fmt.Errorf("locking review: %w", err)
	}
	return s.unit.reviews.findWithExecutor(ctx, s.tx, id)
}
func (s *reviewTransactionalStore) FindReport(ctx context.Context, id int) (*workorder.ReviewReport, error) {
	return s.unit.reports.findWithExecutor(ctx, s.tx, id)
}
func (s *reviewTransactionalStore) SaveReview(ctx context.Context, id int, r *workorder.Review) error {
	return s.unit.reviews.saveWithExecutor(ctx, s.tx, id, r)
}
func (s *reviewTransactionalStore) SaveReport(ctx context.Context, r *workorder.ReviewReport) error {
	return s.unit.reports.saveWithExecutor(ctx, s.tx, r)
}
func (s *reviewTransactionalStore) SaveDecision(ctx context.Context, d *workorder.ReviewDecision) error {
	return s.unit.reviews.saveDecisionWithExecutor(ctx, s.tx, d)
}
func (s *reviewTransactionalStore) SaveAuditEvent(ctx context.Context, e *audit.Event) error {
	return s.unit.events.saveWithExecutor(ctx, s.tx, e)
}
