package readmodel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetailProposalDerivesOwnRemainingBookingTerms(t *testing.T) {
	first := DetailProposal{AmountCents: 8500000, DepositCents: 1700000,
		PlatformFeeTotalCents: 500000, PlatformFeeDueNowCents: 100000}
	second := DetailProposal{AmountCents: 12000000, DepositCents: 2400000,
		PlatformFeeTotalCents: 800000, PlatformFeeDueNowCents: 300000}

	require.Equal(t, int64(6800000), first.ServiceBalanceCents())
	require.Equal(t, int64(400000), first.PlatformFeeBalanceCents())
	require.Equal(t, int64(9600000), second.ServiceBalanceCents())
	require.Equal(t, int64(500000), second.PlatformFeeBalanceCents())
}
