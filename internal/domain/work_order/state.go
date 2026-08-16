package workorder

import "time"

type state interface {
	status() Status
	completionReport() *CompletionReport
	paidOn() time.Time
	reportCompletion(*CompletionReport) (state, error)
	authorizeBalanceCheckout() error
	registerApprovedBalancePayment(time.Time) (state, error)
}

type baseState struct {
	currentStatus Status
	report        *CompletionReport
	completedOn   time.Time
}

func (s baseState) status() Status {
	return s.currentStatus
}

func (s baseState) completionReport() *CompletionReport {
	return s.report
}

func (s baseState) paidOn() time.Time {
	return s.completedOn
}

func (baseState) reportCompletion(*CompletionReport) (state, error) {
	return nil, ErrCompletionReportAlreadyExists
}

func (baseState) authorizeBalanceCheckout() error {
	return ErrWorkOrderNotAwaitingPayment
}

func (baseState) registerApprovedBalancePayment(time.Time) (state, error) {
	return nil, ErrWorkOrderNotEligibleForFullPayment
}

type scheduledState struct {
	baseState
}

func newScheduledState() state {
	return scheduledState{
		baseState: baseState{currentStatus: StatusScheduled},
	}
}

func (scheduledState) reportCompletion(report *CompletionReport) (state, error) {
	if report == nil {
		return nil, ErrCompletionReportRequired
	}
	return newAwaitingPaymentState(report), nil
}

type paidState struct {
	baseState
}

func newPaidState(report *CompletionReport, paidOn time.Time) state {
	return paidState{
		baseState: baseState{currentStatus: StatusPaid, report: report, completedOn: paidOn},
	}
}

type awaitingPaymentState struct {
	baseState
}

func newAwaitingPaymentState(report *CompletionReport) state {
	return awaitingPaymentState{
		baseState: baseState{currentStatus: StatusAwaitingPayment, report: report},
	}
}

func (awaitingPaymentState) reportCompletion(*CompletionReport) (state, error) {
	return nil, ErrCompletionReportAlreadyExists
}

func (awaitingPaymentState) authorizeBalanceCheckout() error {
	return nil
}

func (s awaitingPaymentState) registerApprovedBalancePayment(paidOn time.Time) (state, error) {
	if paidOn.IsZero() {
		return nil, ErrPaidOnRequired
	}
	return newPaidState(s.report, paidOn.UTC()), nil
}

func (paidState) reportCompletion(*CompletionReport) (state, error) {
	return nil, ErrCompletionReportAlreadyExists
}

func (paidState) authorizeBalanceCheckout() error {
	return ErrWorkOrderAlreadyPaid
}

func (paidState) registerApprovedBalancePayment(time.Time) (state, error) {
	return nil, ErrWorkOrderAlreadyPaid
}
