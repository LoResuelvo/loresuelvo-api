package workorder

import "time"

type state interface {
	status() Status
	completionReport() *CompletionReport
	paidOn() time.Time
	review() *Review
	reportCompletion(*CompletionReport) (state, error)
	authorizeBalanceCheckout() error
	registerApprovedBalancePayment(time.Time) (state, error)
	addReview(*Review) (state, error)
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

func (baseState) review() *Review {
	return nil
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

func (baseState) addReview(*Review) (state, error) {
	return nil, ErrWorkOrderNotPaid
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
	reviewValue *Review
}

func newPaidState(report *CompletionReport, paidOn time.Time, review *Review) state {
	return paidState{
		baseState:   baseState{currentStatus: StatusPaid, report: report, completedOn: paidOn},
		reviewValue: review,
	}
}

func (s paidState) review() *Review {
	return s.reviewValue
}

func (s paidState) addReview(review *Review) (state, error) {
	if s.reviewValue != nil {
		return nil, ErrReviewAlreadyExists
	}
	return newPaidState(s.report, s.completedOn, review), nil
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
	return newPaidState(s.report, paidOn.UTC(), nil), nil
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
