package repositories_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/require"
)

type adminPaymentReaderFixture struct {
	reader             *repositories.AdminPaymentReader
	ids                []string
	proposals          []int
	states             []string
	consumer, provider int
}

func newAdminPaymentReaderFixture(t *testing.T) adminPaymentReaderFixture {
	t.Helper()
	f := newOperationInboxFixture(t)
	consumer, provider := savedJobRequestParticipants(t, f.testContext)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, consumer, provider, now, jobrequest.StatusAccepted)
	proposal := f.proposal(t, request, now, serviceproposal.StatusAccepted)
	states := []string{"requires_checkout", "checkout_ready", "processing", "paid", "rejected", "expired", "cancelled", "refunded", "disputed", "payment_mismatch"}
	fixture := adminPaymentReaderFixture{reader: repositories.NewAdminPaymentReader(f.testContext.database), ids: make([]string, len(states)), proposals: make([]int, len(states)), states: states, consumer: consumer, provider: provider}
	// Active states cannot coexist for one proposal/purpose, so historical fixtures isolate proposals.
	for n, state := range states {
		p := proposal
		if n < 4 && n > 0 {
			p = f.proposal(t, request, now, serviceproposal.StatusAccepted)
		}
		fixture.proposals[n] = p
		fixture.ids[n] = fmt.Sprintf("00000000-0000-4000-8000-%012d", n+1)
		_, err := f.testContext.database.Exec(`INSERT INTO payment_intents(id,service_proposal_id,purpose,currency,seller_amount_cents,platform_fee_cents,total_amount_cents,status,created_on,updated_on) VALUES($1,$2,'booking_deposit','ARS',100,10,110,$3,$4,$4)`, fixture.ids[n], p, state, now)
		require.NoError(t, err)
	}
	for n, status := range []string{"approved", "refunded", "disputed"} {
		_, err := f.testContext.database.Exec(`INSERT INTO payment_transactions(payment_intent_id,processor,external_payment_id,seller_account_id,status,currency,amount_cents,verified_on,created_on,updated_on) VALUES($1,'mercado_pago',$2,'private-seller',$3,'ARS',110,$4,$4,$4)`, fixture.ids[9], fmt.Sprintf("payment-%d", n), status, now)
		require.NoError(t, err)
	}
	return fixture
}
func TestAdminPaymentReaderFiltersAllPersistedStatuses(t *testing.T) {
	f := newAdminPaymentReaderFixture(t)
	for n, state := range f.states {
		snapshot, err := f.reader.FindPage(t.Context(), payment.AdminPaymentQuery{Limit: 20, IntentStatus: state})
		require.NoError(t, err)
		require.Len(t, snapshot.Payments, 1)
		require.Equal(t, f.ids[n], snapshot.Payments[0].ID)
	}
}
func TestAdminPaymentReaderKeysetOrdersUUIDTiesWithoutRepeating(t *testing.T) {
	f := newAdminPaymentReaderFixture(t)
	q := payment.AdminPaymentQuery{Limit: 2}
	got := []string{}
	for {
		s, err := f.reader.FindPage(t.Context(), q)
		require.NoError(t, err)
		for _, p := range s.Payments {
			got = append(got, p.ID)
		}
		if !s.HasMore {
			break
		}
		last := s.Payments[len(s.Payments)-1]
		q.After = &rm.AdminPaymentPosition{CreatedOn: last.CreatedOn, ID: last.ID}
	}
	require.Len(t, got, 10)
	for n, id := range got {
		require.Equal(t, f.ids[9-n], id)
	}
}
func TestAdminPaymentReaderKeepsWholeProposalEvidenceBeyondExternalFilter(t *testing.T) {
	f := newAdminPaymentReaderFixture(t)
	s, err := f.reader.FindPage(t.Context(), payment.AdminPaymentQuery{Limit: 1, ExternalPaymentID: "payment-0"})
	require.NoError(t, err)
	require.Len(t, s.Payments, 1)
	require.Len(t, s.Payments[0].Transactions, 3)
	require.Equal(t, []string{"approved", "refunded", "disputed"}, []string{s.Payments[0].Transactions[0].Status, s.Payments[0].Transactions[1].Status, s.Payments[0].Transactions[2].Status})
	require.Len(t, s.Proposals, 1)
	require.Equal(t, f.proposals[9], s.Proposals[0].ID)
	require.Greater(t, len(s.Proposals[0].Intents), 1)
	require.Equal(t, f.consumer, s.Payments[0].ConsumerID)
	require.Equal(t, f.provider, s.Payments[0].ProviderID)
}
func TestAdminPaymentReaderDoesNotTreatExactExternalIDAsPrefix(t *testing.T) {
	f := newAdminPaymentReaderFixture(t)
	empty, err := f.reader.FindPage(t.Context(), payment.AdminPaymentQuery{Limit: 20, ExternalPaymentID: "payment-"})
	require.NoError(t, err)
	require.Empty(t, empty.Payments)
}
func TestAdminPaymentReaderTreatsEmailSearchWildcardsLiterally(t *testing.T) {
	f := newAdminPaymentReaderFixture(t)
	for _, q := range []payment.AdminPaymentQuery{{Limit: 20, ConsumerEmail: "%"}, {Limit: 20, ProviderEmail: "_"}} {
		empty, err := f.reader.FindPage(t.Context(), q)
		require.NoError(t, err)
		require.Empty(t, empty.Payments)
	}
}
func TestAdminPaymentReaderCombinesProposalAndStatusUsingAND(t *testing.T) {
	f := newAdminPaymentReaderFixture(t)
	empty, err := f.reader.FindPage(t.Context(), payment.AdminPaymentQuery{Limit: 20, ServiceProposalID: f.proposals[1], IntentStatus: "refunded"})
	require.NoError(t, err)
	require.Empty(t, empty.Payments)
}

func TestAdminPaymentReaderTimestampBoundsPreserveNanosecondContract(t *testing.T) {
	f := newOperationInboxFixture(t)
	consumer, provider := savedJobRequestParticipants(t, f.testContext)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	request := f.jobRequest(t, consumer, provider, now, jobrequest.StatusAccepted)
	proposal := f.proposal(t, request, now, serviceproposal.StatusAccepted)
	for n := 0; n < 2; n++ {
		_, err := f.testContext.database.Exec(`INSERT INTO payment_intents(id,service_proposal_id,purpose,currency,seller_amount_cents,platform_fee_cents,total_amount_cents,status,created_on,updated_on) VALUES($1,$2,'booking_deposit','ARS',100,10,110,'rejected',$3,$3)`, fmt.Sprintf("00000000-0000-4000-8000-%012d", n+1), proposal, now.Add(time.Duration(n)*time.Microsecond))
		require.NoError(t, err)
	}
	bound := now.Add(time.Nanosecond)
	r := repositories.NewAdminPaymentReader(f.testContext.database)
	s, err := r.FindPage(t.Context(), payment.AdminPaymentQuery{Limit: 20, CreatedFrom: &bound})
	require.NoError(t, err)
	require.Len(t, s.Payments, 1)
	require.Equal(t, now.Add(time.Microsecond), s.Payments[0].CreatedOn)
	s, err = r.FindPage(t.Context(), payment.AdminPaymentQuery{Limit: 20, CreatedTo: &bound})
	require.NoError(t, err)
	require.Len(t, s.Payments, 1)
	require.Equal(t, now, s.Payments[0].CreatedOn)
}
