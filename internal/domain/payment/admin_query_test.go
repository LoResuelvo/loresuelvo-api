package payment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminPaymentQueryHistoricalStatuses(t *testing.T) {
	for _, s := range []string{"requires_checkout", "checkout_ready", "processing", "paid", "rejected", "expired", "cancelled", "refunded", "disputed", "payment_mismatch"} {
		require.NoError(t, (AdminPaymentQuery{Limit: 20, IntentStatus: s}).Validate())
	}
	for _, q := range []AdminPaymentQuery{{Limit: 0}, {Limit: 101}, {Limit: 20, IntentStatus: "approved"}, {Limit: 20, PaymentIntentID: "invalid"}, {Limit: 20, Purpose: "refund"}, {Limit: 20, ServiceProposalID: -1}} {
		require.ErrorIs(t, q.Validate(), ErrInvalidAdminPaymentQuery)
	}
}
