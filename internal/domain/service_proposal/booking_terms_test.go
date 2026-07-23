package serviceproposal_test

import (
	"testing"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookingPolicyCalculatesProposalTerms(t *testing.T) {
	policy := serviceproposal.NewBookingPolicy()

	terms, err := policy.Calculate(10000000)

	require.NoError(t, err)
	assert.Equal(t, "ARS", terms.Currency())
	assert.Equal(t, int64(10000000), terms.ServiceTotalCents())
	assert.Equal(t, int64(2000000), terms.DepositCents())
	assert.Equal(t, int64(8000000), terms.RemainingServiceBalanceCents())
	assert.Equal(t, int64(500000), terms.PlatformFeeTotalCents())
	assert.Equal(t, int64(100000), terms.PlatformFeeDueNowCents())
	assert.Equal(t, int64(400000), terms.RemainingPlatformFeeCents())
	assert.Equal(t, int64(2100000), terms.AmountDueNowCents())
	assert.Equal(t, int64(8400000), terms.RemainingAmountDueCents())
	assert.Equal(t, int64(10500000), terms.ContractTotalCents())
}

func TestBookingPolicyRoundsInitialAmountsToNearestCent(t *testing.T) {
	policy := serviceproposal.NewBookingPolicy()

	terms, err := policy.Calculate(10000003)

	require.NoError(t, err)
	assert.Equal(t, int64(2000001), terms.DepositCents())
	assert.Equal(t, int64(100000), terms.PlatformFeeDueNowCents())
	assert.Equal(t, int64(2100001), terms.AmountDueNowCents())
	assert.Equal(t, int64(8400002), terms.RemainingAmountDueCents())
	assert.Equal(t, int64(10500003), terms.ContractTotalCents())
	assert.Equal(t, terms.ContractTotalCents(), terms.AmountDueNowCents()+terms.RemainingAmountDueCents())
}

func TestBookingPolicyRejectsNonPositiveServiceTotal(t *testing.T) {
	policy := serviceproposal.NewBookingPolicy()

	terms, err := policy.Calculate(0)

	assert.ErrorIs(t, err, serviceproposal.ErrInvalidAmount)
	assert.Equal(t, serviceproposal.BookingTerms{}, terms)
}
