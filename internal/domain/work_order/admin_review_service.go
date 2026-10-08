package workorder

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
	"github.com/google/uuid"
)

type ReviewPageInput struct{ Page, Limit int }

func (p ReviewPageInput) Normalize() (ReviewPageInput, error) {
	if p.Page == 0 {
		p.Page = 1
	}
	if p.Limit == 0 {
		p.Limit = 20
	}
	if p.Page < 1 || p.Limit < 1 || p.Limit > 100 || p.Page > math.MaxInt/p.Limit {
		return ReviewPageInput{}, ErrInvalidReviewModeration
	}
	return p, nil
}

type ReviewListInput struct {
	ReviewPageInput
	Status string
}

func (p ReviewListInput) Normalize() (ReviewListInput, error) {
	page, err := p.ReviewPageInput.Normalize()
	if err != nil {
		return ReviewListInput{}, err
	}
	p.ReviewPageInput = page
	if p.Status == "" {
		p.Status = "reported"
	}
	switch p.Status {
	case "reported", "visible", "hidden", "all":
	default:
		return ReviewListInput{}, ErrInvalidReviewModeration
	}
	return p, nil
}

type AdminReviewPage struct {
	Items       []readmodel.AdminReviewSummary
	Page, Limit int
	Total       int64
}
type AdminReviewDetail struct {
	Summary     readmodel.AdminReviewSummary
	Review      *Review
	Report      *ReviewReport
	Decisions   []*ReviewDecision
	Page, Limit int
	Total       int64
}
type AdminReviewReader interface {
	FindPage(context.Context, ReviewListInput) (*AdminReviewPage, error)
	FindByID(context.Context, int, ReviewPageInput) (*AdminReviewDetail, error)
}
type ReviewModerationResult struct {
	Review   *Review
	Decision *ReviewDecision
	Report   *ReviewReport
}
type AdminReviewService struct {
	reader    AdminReviewReader
	operators audit.OperatorIDFinder
	unit      ReviewUnitOfWork
	events    audit.Writer
	clock     clock.Clock
}

func NewAdminReviewService(reader AdminReviewReader, operators audit.OperatorIDFinder, unit ReviewUnitOfWork, events audit.Writer, clock clock.Clock) *AdminReviewService {
	return &AdminReviewService{reader: reader, operators: operators, unit: unit, events: events, clock: clock}
}
func (s *AdminReviewService) List(ctx context.Context, input ReviewListInput) (*AdminReviewPage, error) {
	normalized, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	return s.reader.FindPage(ctx, normalized)
}
func (s *AdminReviewService) Get(ctx context.Context, auth string, id int, page ReviewPageInput, correlation string) (*AdminReviewDetail, error) {
	normalized, err := page.Normalize()
	if err != nil {
		return nil, err
	}
	operator, err := s.operators.FindOperatorIDByAuthID(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("finding review operator: %w", err)
	}
	if operator <= 0 {
		return nil, ErrReviewReportForbidden
	}
	detail, err := s.reader.FindByID(ctx, id, normalized)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, ErrReviewNotAvailable
	}
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operator, Action: audit.ActionAccess, ResourceType: "review", ResourceID: strconv.Itoa(id), OccurredOn: s.clock.Now(), Result: audit.ResultPrepared, CorrelationID: correlation})
	if err != nil {
		return nil, err
	}
	if err = s.events.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving review access audit: %w", err)
	}
	return detail, nil
}
func (s *AdminReviewService) Moderate(ctx context.Context, auth string, id int, input ModerationInput, correlation string) (*ReviewModerationResult, error) {
	input, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	operator, err := s.operators.FindOperatorIDByAuthID(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("finding review operator: %w", err)
	}
	if operator <= 0 {
		return nil, ErrReviewReportForbidden
	}
	var result *ReviewModerationResult
	err = s.unit.Execute(ctx, func(store ReviewStore) error {
		review, err := store.FindReview(ctx, id)
		if err != nil {
			return err
		}
		report, err := store.FindReport(ctx, id)
		if errors.Is(err, ErrReviewReportNotFound) {
			report = nil
		} else if err != nil {
			return err
		}
		decision, err := review.Moderate(id, input, report, operator, s.clock.Now())
		if err != nil {
			return err
		}
		if err = store.SaveDecision(ctx, decision); err != nil {
			return err
		}
		review.AssociateDecision(decision)
		if err = store.SaveReview(ctx, id, review); err != nil {
			return err
		}
		if decision.ReportID() > 0 {
			if err = store.SaveReport(ctx, report); err != nil {
				return err
			}
		}
		event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operator, Action: audit.ActionExecute, ResourceType: "review_decision", ResourceID: strconv.Itoa(decision.ID()), OccurredOn: decision.CreatedOn(), Result: audit.ResultSucceeded, CorrelationID: correlation})
		if err != nil {
			return err
		}
		if err = store.SaveAuditEvent(ctx, event); err != nil {
			return err
		}
		result = &ReviewModerationResult{Review: review, Decision: decision, Report: report}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("saving review moderation: %w", err)
	}
	return result, nil
}
