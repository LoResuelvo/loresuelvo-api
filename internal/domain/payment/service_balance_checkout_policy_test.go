package payment_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
)

func TestServiceBalanceCheckoutPolicyAuthorizesOnlyEligibleConsumerAtScheduledTime(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order := newWorkOrderFixture(t, 84, &serviceproposal.ServiceProposal{
		ID:          42,
		Consumer:    &consumer.Consumer{BaseUser: user.RehydrateBaseUser(10, "", "", "", "", consumer.Role, nil)},
		Provider:    &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "", "", "", "", provider.Role, nil)},
		ScheduledOn: scheduledOn,
	}, scheduledOn.Add(-48*time.Hour))
	policy := payment.ServiceBalanceCheckoutPolicy{}

	assert.ErrorIs(t, policy.Authorize(nil, 10, scheduledOn), payment.ErrInvalidWorkOrder)
	assert.ErrorIs(t, policy.Authorize(order, 11, scheduledOn), payment.ErrOnlyWorkOrderConsumerCanCheckout)
	assert.ErrorIs(t, policy.Authorize(order, 10, scheduledOn.Add(-time.Nanosecond)), payment.ErrServiceBalancePaymentNotAvailable)

	fullyPaid := newWorkOrderFixture(t, 84, order.ServiceProposal().(*serviceproposal.ServiceProposal), scheduledOn.Add(-48*time.Hour))
	assert.NoError(t, fullyPaid.MarkPaid())
	assert.ErrorIs(t, policy.Authorize(fullyPaid, 10, scheduledOn), payment.ErrWorkOrderAlreadyFullyPaid)
	assert.ErrorIs(t, policy.Authorize(fullyPaid, 11, scheduledOn), payment.ErrOnlyWorkOrderConsumerCanCheckout)
	assert.NoError(t, policy.Authorize(order, 10, scheduledOn))
}
