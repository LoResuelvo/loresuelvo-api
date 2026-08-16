package workorder

import (
	"fmt"
	"time"
)

type RestoreFactory struct{}

func (RestoreFactory) Restore(
	id int,
	serviceProposal ServiceProposal,
	status Status,
	acceptedOn time.Time,
) (*WorkOrder, error) {
	if id <= 0 || serviceProposal == nil {
		return nil, ErrInvalidWorkOrderIdentity
	}

	order, err := New(serviceProposal, acceptedOn)
	if err != nil {
		return nil, fmt.Errorf("restoring work order: %w", err)
	}
	order.SetID(id)

	switch status {
	case StatusScheduled:
		return order, nil
	case StatusPaid:
		if err := order.MarkPaid(); err != nil {
			return nil, fmt.Errorf("restoring paid work order: %w", err)
		}
		return order, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidWorkOrderState, status)
	}
}
