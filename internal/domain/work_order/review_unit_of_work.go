package workorder

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
)

type ReviewStore interface {
	FindReview(context.Context, int) (*Review, error)
	FindReport(context.Context, int) (*ReviewReport, error)
	SaveReview(context.Context, int, *Review) error
	SaveReport(context.Context, *ReviewReport) error
	SaveDecision(context.Context, *ReviewDecision) error
	SaveAuditEvent(context.Context, *audit.Event) error
}
type ReviewUnitOfWork interface {
	Execute(context.Context, func(ReviewStore) error) error
}

func RestoreReview(orderID int, input ReviewRestoreInput) (*Review, error) {
	review, err := restoreReview(&input)
	if err != nil {
		return nil, err
	}
	if orderID <= 0 {
		return nil, ErrInvalidWorkOrderIdentity
	}
	review.SetID(orderID)
	return review, nil
}
