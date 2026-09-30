package payment

import (
	"math"
	"testing"
	"time"

	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/stretchr/testify/require"
)

func adminPolicyProposal() rm.AdminPaymentProposal {
	id := 1
	return rm.AdminPaymentProposal{ID: 1, Status: "accepted", WorkOrderID: &id, WorkOrderStatus: "awaiting_payment", Breakdown: rm.AdminPaymentBreakdown{Currency: "ARS", ServiceTotalCents: 3333333, DepositCents: 777777, PlatformFeeTotalCents: 555555, PlatformFeeDueNowCents: 123457, BookingPaymentDeadline: time.Now()}, Intents: []rm.AdminPayment{{ID: "a", Purpose: "booking_deposit", IntentStatus: "paid", Currency: "ARS", SellerAmountCents: 777777, PlatformFeeCents: 123457, TotalAmountCents: 901234, Transactions: []rm.AdminPaymentTransaction{{Processor: "mercado_pago", ExternalPaymentID: "1", Status: "approved", Currency: "ARS", AmountCents: 901234}}}}}
}
func TestAdminPaymentPolicyGrossEvidenceDoesNotSubtractFromContract(t *testing.T) {
	p := adminPolicyProposal()
	p.Intents[0].Transactions = append(p.Intents[0].Transactions, p.Intents[0].Transactions[0], rm.AdminPaymentTransaction{Processor: "mercado_pago", ExternalPaymentID: "2", Status: "approved", Currency: "USD", AmountCents: 901235})
	b, s, flags, err := projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.Equal(t, int64(2987654), *b.RemainingAmountDueCents)
	require.Equal(t, []rm.AdminPaymentAmount{{Currency: "ARS", AmountCents: 901234}, {Currency: "USD", AmountCents: 901235}}, s.ApprovedAmounts)
	require.Equal(t, int64(2987654), s.PendingAmount.AmountCents)
	require.Contains(t, s.Anomalies, "multiple_approved_transactions")
	require.Contains(t, flags["a"], "transaction_currency_mismatch")
}
func TestAdminPaymentPolicyPendingUnavailableWithoutValidDeposit(t *testing.T) {
	p := adminPolicyProposal()
	p.Intents[0].Transactions[0].Currency = "USD"
	_, s, _, err := projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.Nil(t, s.PendingAmount)
	p.WorkOrderStatus = "paid"
	_, s, _, err = projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.Zero(t, s.PendingAmount.AmountCents)
	p.WorkOrderID = nil
	_, s, _, err = projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.Nil(t, s.PendingAmount)
}
func TestAdminPaymentPolicyOverflowFailsClosed(t *testing.T) {
	p := adminPolicyProposal()
	p.Intents[0].Transactions[0].AmountCents = math.MaxInt64
	p.Intents[0].Transactions = append(p.Intents[0].Transactions, rm.AdminPaymentTransaction{Processor: "mercado_pago", ExternalPaymentID: "2", Status: "approved", Currency: "ARS", AmountCents: 1})
	_, _, _, err := projectAdminPaymentProposal(p)
	require.ErrorIs(t, err, ErrAdminPaymentAmountOverflow)
}

func TestAdminPaymentPolicyRejectsUnapprovedEvidenceFromGrossTotals(t *testing.T) {
	p := adminPolicyProposal()
	p.Intents[0].IntentStatus = "processing"
	p.Intents[0].Transactions[0].Status = "processing"
	p.Intents[0].Transactions = append(p.Intents[0].Transactions, rm.AdminPaymentTransaction{Processor: "mercado_pago", ExternalPaymentID: "rejected", Status: "rejected", Currency: "ARS", AmountCents: 901234})
	_, s, _, err := projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.NotNil(t, s.ApprovedAmounts)
	require.Empty(t, s.ApprovedAmounts)
	require.Nil(t, s.PendingAmount)
}
func TestAdminPaymentPolicyFlagsPaidIntentMissingApprovedEvidence(t *testing.T) {
	p := adminPolicyProposal()
	p.Intents[0].Transactions = nil
	_, s, flags, err := projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.Contains(t, flags["a"], "paid_without_matching_approved_transaction")
	require.Empty(t, s.ApprovedAmounts)
}
func TestAdminPaymentPolicyContractOverflowFailsClosed(t *testing.T) {
	p := adminPolicyProposal()
	p.Breakdown.ServiceTotalCents = math.MaxInt64
	_, _, _, err := projectAdminPaymentProposal(p)
	require.ErrorIs(t, err, ErrAdminPaymentAmountOverflow)
}
func TestAdminPaymentPolicySameCurrencyExcessPreservesNominalPending(t *testing.T) {
	p := adminPolicyProposal()
	p.Intents[0].Transactions = append(p.Intents[0].Transactions, rm.AdminPaymentTransaction{Processor: "mercado_pago", ExternalPaymentID: "2", Status: "approved", Currency: "ARS", AmountCents: 901234})
	_, s, flags, err := projectAdminPaymentProposal(p)
	require.NoError(t, err)
	require.Equal(t, int64(1802468), s.ApprovedAmounts[0].AmountCents)
	require.Equal(t, int64(2987654), s.PendingAmount.AmountCents)
	require.Contains(t, flags["a"], "approved_amount_exceeds_intent_total")
}
