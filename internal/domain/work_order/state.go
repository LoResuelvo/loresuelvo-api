package workorder

type state interface {
	status() Status
	markPaid() (state, error)
}

type baseState struct {
	currentStatus Status
}

func (s baseState) status() Status {
	return s.currentStatus
}

func (baseState) markPaid() (state, error) {
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

func (scheduledState) markPaid() (state, error) {
	return newPaidState(), nil
}

type paidState struct {
	baseState
}

func newPaidState() state {
	return paidState{
		baseState: baseState{currentStatus: StatusPaid},
	}
}
