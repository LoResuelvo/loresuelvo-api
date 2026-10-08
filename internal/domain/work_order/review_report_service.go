package workorder

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type ReportReviewService struct {
	actors ReviewReportActorFinder
	orders ReviewReportOrderFinder
	unit   ReviewUnitOfWork
	clock  clock.Clock
}

func NewReportReviewService(actors ReviewReportActorFinder, orders ReviewReportOrderFinder, unit ReviewUnitOfWork, clock clock.Clock) *ReportReviewService {
	return &ReportReviewService{actors: actors, orders: orders, unit: unit, clock: clock}
}
func (s *ReportReviewService) Report(ctx context.Context, authID string, orderID int, category, explanation string) (*ReviewReport, error) {
	actorID, role, err := s.actors.FindByAuthID(ctx, authID)
	if errors.Is(err, user.ErrNotFound) {
		return nil, ErrReviewReportForbidden
	}
	if err != nil {
		return nil, fmt.Errorf("finding review reporter: %w", err)
	}
	if role != provider.Role || actorID <= 0 {
		return nil, ErrReviewReportForbidden
	}
	order, err := s.orders.FindByID(ctx, orderID)
	if errors.Is(err, ErrDoesNotExist) || (err == nil && order == nil) {
		return nil, ErrReviewNotAvailable
	}
	if err != nil {
		return nil, fmt.Errorf("finding reported work order: %w", err)
	}
	if err = order.AuthorizeReviewReporter(actorID); err != nil {
		return nil, err
	}

	var result *ReviewReport
	err = s.unit.Execute(ctx, func(store ReviewStore) error {
		review, err := store.FindReview(ctx, orderID)
		if err != nil {
			return err
		}
		_, err = store.FindReport(ctx, orderID)
		if err == nil {
			return ErrReviewReportAlreadyExists
		}
		if !errors.Is(err, ErrReviewReportNotFound) {
			return fmt.Errorf("finding existing review report: %w", err)
		}
		result, err = review.NewReport(orderID, actorID, category, explanation, s.clock.Now())
		if err != nil {
			return err
		}
		return store.SaveReport(ctx, result)
	})
	if err != nil {
		return nil, fmt.Errorf("saving review report: %w", err)
	}
	return result, nil
}
