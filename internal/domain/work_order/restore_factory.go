package workorder

import (
	"fmt"
	"time"
)

type RestoreFactory struct{}

type RestoreInput struct {
	ID               int
	ServiceProposal  ServiceProposal
	Status           Status
	AcceptedOn       time.Time
	CompletionReport *CompletionReportRestoreInput
	PaidOn           time.Time
}

type CompletionReportRestoreInput struct {
	ID           int
	Description  string
	ImageFileIDs []string
	ReportedOn   time.Time
}

func (RestoreFactory) Restore(input RestoreInput) (*WorkOrder, error) {
	if input.ID <= 0 || input.ServiceProposal == nil {
		return nil, ErrInvalidWorkOrderIdentity
	}

	order, err := New(input.ServiceProposal, input.AcceptedOn)
	if err != nil {
		return nil, fmt.Errorf("restoring work order: %w", err)
	}
	order.SetID(input.ID)

	completionReport, err := restoreCompletionReport(input.CompletionReport)
	if err != nil {
		return nil, err
	}

	switch input.Status {
	case StatusScheduled:
		if completionReport != nil || !input.PaidOn.IsZero() {
			return nil, fmt.Errorf("%w: scheduled work order cannot contain completion evidence or paid_on", ErrInvalidWorkOrderState)
		}
		return order, nil
	case StatusAwaitingPayment:
		if completionReport == nil {
			return nil, fmt.Errorf("%w: awaiting payment work order requires completion report", ErrInvalidWorkOrderState)
		}
		order.state = newAwaitingPaymentState(completionReport)
		return order, nil
	case StatusPaid:
		if completionReport == nil || input.PaidOn.IsZero() {
			return nil, fmt.Errorf("%w: paid work order requires completion report and paid_on", ErrInvalidWorkOrderState)
		}
		order.state = newPaidState(completionReport, input.PaidOn.UTC())
		return order, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidWorkOrderState, input.Status)
	}
}

func restoreCompletionReport(input *CompletionReportRestoreInput) (*CompletionReport, error) {
	if input == nil {
		return nil, nil
	}
	if input.ID <= 0 {
		return nil, fmt.Errorf("restoring completion report: %w", ErrInvalidWorkOrderIdentity)
	}

	report, err := NewCompletionReport(input.Description, input.ImageFileIDs, input.ReportedOn)
	if err != nil {
		return nil, fmt.Errorf("restoring completion report: %w", err)
	}
	report.SetID(input.ID)
	return report, nil
}
