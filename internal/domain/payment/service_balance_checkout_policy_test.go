package payment_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
)

func TestServiceBalanceCheckoutPolicyAuthorizesOnlyEligibleConsumerAtScheduledTime(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order := &workorder.WorkOrder{
		ID: 84,
		ServiceProposal: &serviceproposal.ServiceProposal{
			ID:          42,
			Consumer:    &consumer.Consumer{BaseUser: user.RehydrateBaseUser(10, "", "", "", "", consumer.Role, nil)},
			Provider:    &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "", "", "", "", provider.Role, nil)},
			ScheduledOn: scheduledOn,
		},
		Status: workorder.StatusScheduled,
	}
	policy := payment.ServiceBalanceCheckoutPolicy{}

	assert.ErrorIs(t, policy.Authorize(nil, 10, scheduledOn), payment.ErrInvalidWorkOrder)
	assert.ErrorIs(t, policy.Authorize(order, 11, scheduledOn), payment.ErrOnlyWorkOrderConsumerCanCheckout)
	assert.ErrorIs(t, policy.Authorize(order, 10, scheduledOn.Add(-time.Nanosecond)), payment.ErrServiceBalancePaymentNotAvailable)

	nonScheduled := *order
	nonScheduled.Status = workorder.Status("completed")
	assert.ErrorIs(t, policy.Authorize(&nonScheduled, 10, scheduledOn), payment.ErrWorkOrderNotScheduled)

	fullyPaid := *order
	fullyPaid.Status = workorder.StatusPaid
	assert.ErrorIs(t, policy.Authorize(&fullyPaid, 10, scheduledOn), payment.ErrWorkOrderAlreadyFullyPaid)
	assert.ErrorIs(t, policy.Authorize(&fullyPaid, 11, scheduledOn), payment.ErrOnlyWorkOrderConsumerCanCheckout)
	assert.NoError(t, policy.Authorize(order, 10, scheduledOn))
}
