package payment

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/google/uuid"
)

type AdminPaymentReader interface {
	FindPage(context.Context, AdminPaymentQuery) (*rm.AdminPaymentSnapshot, error)
}
type AdminPaymentService struct {
	reader    AdminPaymentReader
	operators audit.OperatorIDFinder
	writer    audit.Writer
	clock     clock.Clock
}

func NewAdminPaymentService(reader AdminPaymentReader, operators audit.OperatorIDFinder, writer audit.Writer, clock clock.Clock) *AdminPaymentService {
	return &AdminPaymentService{reader, operators, writer, clock}
}
func (s *AdminPaymentService) Query(ctx context.Context, q AdminPaymentQuery, subject, correlation string) (*rm.AdminPaymentPage, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	snapshot, err := s.reader.FindPage(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading administrative payments: %w", err)
	}
	if snapshot == nil {
		return nil, fmt.Errorf("reading administrative payments: missing snapshot")
	}
	type projection struct {
		breakdown rm.AdminPaymentBreakdown
		summary   rm.AdminPaymentSummary
		flags     map[string][]string
	}
	projected := make(map[int]projection, len(snapshot.Proposals))
	for _, p := range snapshot.Proposals {
		b, summary, flags, e := projectAdminPaymentProposal(p)
		if e != nil {
			return nil, fmt.Errorf("projecting administrative payments: %w", e)
		}
		projected[p.ID] = projection{b, summary, flags}
	}
	page := &rm.AdminPaymentPage{Payments: make([]rm.AdminPayment, 0, len(snapshot.Payments)), Limit: q.Limit}
	for _, p := range snapshot.Payments {
		projection, ok := projected[p.ServiceProposalID]
		if !ok {
			return nil, fmt.Errorf("projecting administrative payments: missing proposal")
		}
		p.Breakdown = projection.breakdown
		p.Summary = projection.summary
		p.Anomalies = projection.flags[p.ID]
		if p.Transactions == nil {
			p.Transactions = make([]rm.AdminPaymentTransaction, 0)
		}
		if p.Anomalies == nil {
			p.Anomalies = make([]string, 0)
		}
		page.Payments = append(page.Payments, p)
	}
	if snapshot.HasMore && len(page.Payments) > 0 {
		last := page.Payments[len(page.Payments)-1]
		page.Next = &rm.AdminPaymentPosition{CreatedOn: last.CreatedOn, ID: last.ID}
	}
	operator, err := s.operators.FindOperatorIDByAuthID(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("resolving payment access operator: %w", err)
	}
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operator, Action: audit.ActionAccess, ResourceType: "payment", OccurredOn: s.clock.Now(), Result: audit.ResultPrepared, CorrelationID: correlation})
	if err != nil {
		return nil, fmt.Errorf("preparing payment access audit: %w", err)
	}
	if err := s.writer.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving payment access audit: %w", err)
	}
	return page, nil
}
