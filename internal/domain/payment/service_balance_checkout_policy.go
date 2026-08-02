package payment

import (
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type ServiceBalanceCheckoutPolicy struct{}

func (ServiceBalanceCheckoutPolicy) Authorize(
	order *workorder.WorkOrder,
	consumerID int,
	now time.Time,
) error {
	if order == nil || order.ServiceProposal == nil {
		return ErrInvalidWorkOrder
	}
	if consumerID <= 0 || order.ConsumerID() != consumerID {
		return ErrOnlyWorkOrderConsumerCanCheckout
	}
	if order.Status == workorder.StatusPaid {
		return ErrWorkOrderAlreadyFullyPaid
	}
	if order.Status != workorder.StatusScheduled {
		return ErrWorkOrderNotScheduled
	}
	if now.IsZero() || now.UTC().Before(order.ScheduledOn().UTC()) {
		return ErrServiceBalancePaymentNotAvailable
	}
	return nil
}
