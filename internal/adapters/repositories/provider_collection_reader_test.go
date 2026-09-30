package repositories_test

import (
	"math"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderCollectionReaderReconcilesSummaryAndFilteredPagesWithoutPaymentAccount(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-reconcile")
	other := newProviderWorkOrderTestFixture(t, testContext, "collection-other")
	base := time.Now().UTC().Truncate(time.Microsecond)
	first := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(48*time.Hour), uuid.NewString())
	second := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(72*time.Hour), uuid.NewString())
	foreign := savePaidWorkOrderWithoutReviewForFixture(t, testContext, other, base.Add(96*time.Hour), uuid.NewString())
	setCollectionTerms(t, testContext, first.ServiceProposalID(), 100000, 20000, 5000, 1000)
	setCollectionTerms(t, testContext, second.ServiceProposalID(), 100000, 20000, 5000, 1000)
	setCollectionTerms(t, testContext, foreign.ServiceProposalID(), 100000, 20000, 5000, 1000)
	firstDeposit := saveCollectionPayment(t, testContext, first.ServiceProposalID(), "booking_deposit", 20000, 1000, base.Add(20*time.Hour))
	secondDeposit := saveCollectionPayment(t, testContext, second.ServiceProposalID(), "booking_deposit", 20000, 1000, base.Add(21*time.Hour))
	firstBalance := saveCollectionPayment(t, testContext, first.ServiceProposalID(), "service_balance", 80000, 4000, base.Add(22*time.Hour))
	saveCollectionPayment(t, testContext, foreign.ServiceProposalID(), "booking_deposit", 20000, 1000, base.Add(21*time.Hour))
	addActivityCompletionImages(t, testContext, fixture.providerAuthID, first.ID(), 2)
	reader := repositories.NewProviderCollectionReader(testContext.database)
	period := provider.ActivityQuery{From: base.Add(19 * time.Hour), To: base.Add(23 * time.Hour), Granularity: provider.ActivityDay}

	summary, err := reader.ReadSummary(t.Context(), fixture.providerID, period)
	require.NoError(t, err)
	require.Equal(t, int64(40000), summary.Current.BookingDepositCents)
	require.Equal(t, int64(80000), summary.Current.ServiceBalanceCents)
	require.Zero(t, summary.Pending.AwaitingPayment.Orders)
	var depositSeries, balanceSeries int64
	for _, bucket := range summary.Series {
		depositSeries += bucket.BookingDepositCents
		balanceSeries += bucket.ServiceBalanceCents
	}
	require.Equal(t, summary.Current.BookingDepositCents, depositSeries)
	require.Equal(t, summary.Current.ServiceBalanceCents, balanceSeries)

	all, err := reader.ReadDetail(t.Context(), fixture.providerID, provider.CollectionDetailQuery{Period: period, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(3), all.TotalCount)
	require.Equal(t, int64(120000), all.TotalAmountCents)
	require.Len(t, all.Page, 3)
	require.Equal(t, firstBalance, all.Page[0].ID)
	require.Equal(t, secondDeposit, all.Page[1].ID)
	require.Equal(t, firstDeposit, all.Page[2].ID)
	require.Equal(t, first.ID(), *all.Page[0].WorkOrderID)
	require.Nil(t, all.Next)

	filtered := provider.CollectionDetailQuery{Period: period, Purpose: provider.CollectionBookingDeposit, Limit: 1}
	firstPage, err := reader.ReadDetail(t.Context(), fixture.providerID, filtered)
	require.NoError(t, err)
	require.Equal(t, int64(2), firstPage.TotalCount)
	require.Equal(t, int64(40000), firstPage.TotalAmountCents)
	require.Len(t, firstPage.Page, 1)
	require.Equal(t, secondDeposit, firstPage.Page[0].ID)
	require.NotNil(t, firstPage.Next)
	filtered.After = firstPage.Next
	secondPage, err := reader.ReadDetail(t.Context(), fixture.providerID, filtered)
	require.NoError(t, err)
	require.Equal(t, int64(2), secondPage.TotalCount)
	require.Equal(t, int64(40000), secondPage.TotalAmountCents)
	require.Len(t, secondPage.Page, 1)
	require.Equal(t, firstDeposit, secondPage.Page[0].ID)
	require.Nil(t, secondPage.Next)
}

func TestProviderCollectionReaderUsesVerifiedOnAndComparison(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-verified")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(48*time.Hour), uuid.NewString())
	setCollectionTerms(t, testContext, order.ServiceProposalID(), 100000, 20000, 5000, 1000)
	saveCollectionPayment(t, testContext, order.ServiceProposalID(), "booking_deposit", 20000, 1000, base.Add(20*time.Hour))
	saveCollectionPayment(t, testContext, order.ServiceProposalID(), "service_balance", 80000, 4000, base.Add(22*time.Hour))
	reader := repositories.NewProviderCollectionReader(testContext.database)
	query := provider.ActivityQuery{From: base.Add(21 * time.Hour), To: base.Add(23 * time.Hour), Granularity: provider.ActivityDay, ComparePrevious: true}
	snapshot, err := reader.ReadSummary(t.Context(), fixture.providerID, query)
	require.NoError(t, err)
	require.Zero(t, snapshot.Current.BookingDepositCents)
	require.Equal(t, int64(80000), snapshot.Current.ServiceBalanceCents)
	require.NotNil(t, snapshot.Previous)
	require.Equal(t, int64(20000), snapshot.Previous.BookingDepositCents)
	require.Zero(t, snapshot.Previous.ServiceBalanceCents)
}

func TestProviderCollectionReaderPendingBalancesIgnorePeriodAndCheckout(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-pending")
	base := time.Now().UTC().Truncate(time.Microsecond)
	scheduled := saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, base.Add(48*time.Hour))
	awaiting := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(72*time.Hour), uuid.NewString())
	paid := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(96*time.Hour), uuid.NewString())
	for _, order := range []int{scheduled.ServiceProposalID(), awaiting.ServiceProposalID(), paid.ServiceProposalID()} {
		setCollectionTerms(t, testContext, order, 100000, 20000, 5000, 1000)
	}
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET status = 'awaiting_payment', paid_on = NULL WHERE id = $1`, awaiting.ID())
	require.NoError(t, err)
	reader := repositories.NewProviderCollectionReader(testContext.database)
	snapshot, err := reader.ReadSummary(t.Context(), fixture.providerID, provider.ActivityQuery{From: base.Add(-time.Hour), To: base, Granularity: provider.ActivityDay})
	require.NoError(t, err)
	require.Zero(t, snapshot.Current.BookingDepositCents)
	require.Zero(t, snapshot.Current.ServiceBalanceCents)
	require.Equal(t, int64(1), snapshot.Pending.Scheduled.Orders)
	require.Equal(t, int64(80000), snapshot.Pending.Scheduled.AmountCents)
	require.Equal(t, int64(1), snapshot.Pending.AwaitingPayment.Orders)
	require.Equal(t, int64(80000), snapshot.Pending.AwaitingPayment.AmountCents)
}

func TestProviderCollectionReaderFailsOnInconsistentApprovedAmount(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-invalid")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(48*time.Hour), uuid.NewString())
	setCollectionTerms(t, testContext, order.ServiceProposalID(), 100000, 20000, 5000, 1000)
	transactionID := saveCollectionPayment(t, testContext, order.ServiceProposalID(), "booking_deposit", 20000, 1000, base.Add(20*time.Hour))
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE payment_transactions SET amount_cents = amount_cents + 1 WHERE id = $1`, transactionID)
	require.NoError(t, err)
	reader := repositories.NewProviderCollectionReader(testContext.database)
	period := provider.ActivityQuery{From: base.Add(19 * time.Hour), To: base.Add(21 * time.Hour), Granularity: provider.ActivityDay}
	_, err = reader.ReadSummary(t.Context(), fixture.providerID, period)
	require.ErrorContains(t, err, "inconsistent")
	_, err = reader.ReadDetail(t.Context(), fixture.providerID, provider.CollectionDetailQuery{Period: period, Limit: 20})
	require.ErrorContains(t, err, "inconsistent")
}

func TestProviderCollectionReaderRejectsAggregateOverflow(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-overflow")
	base := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 2; i++ {
		order := saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, base.Add(time.Duration(i+48)*time.Hour))
		setCollectionTerms(t, testContext, order.ServiceProposalID(), math.MaxInt64, math.MaxInt64, 0, 0)
		saveCollectionPayment(t, testContext, order.ServiceProposalID(), "booking_deposit", math.MaxInt64, 0, base.Add(time.Duration(i+20)*time.Hour))
	}
	reader := repositories.NewProviderCollectionReader(testContext.database)
	period := provider.ActivityQuery{From: base.Add(19 * time.Hour), To: base.Add(23 * time.Hour), Granularity: provider.ActivityDay}
	_, err := reader.ReadSummary(t.Context(), fixture.providerID, period)
	require.ErrorContains(t, err, "outside the supported integer range")
}

func setCollectionTerms(t *testing.T, testContext serviceProposalRepositoryTestContext, proposalID int, amount, deposit, fee, feeDueNow int64) {
	t.Helper()
	_, err := testContext.database.ExecContext(t.Context(), `
		UPDATE service_proposals SET status = 'accepted', amount_cents = $2, deposit_cents = $3,
			platform_fee_total_cents = $4, platform_fee_due_now_cents = $5
		WHERE id = $1`, proposalID, amount, deposit, fee, feeDueNow)
	require.NoError(t, err)
}

func saveCollectionPayment(t *testing.T, testContext serviceProposalRepositoryTestContext, proposalID int, purpose string, sellerAmount, fee int64, verifiedOn time.Time) int64 {
	t.Helper()
	intent := &payment.Intent{
		ID: uuid.NewString(), ServiceProposalID: proposalID, Purpose: payment.Purpose(purpose),
		Currency: "ARS", SellerAmountCents: sellerAmount, PlatformFeeCents: fee,
		TotalAmountCents: sellerAmount + fee, Status: payment.StatusPaid,
		CreatedOn: verifiedOn.Add(-time.Minute), UpdatedOn: verifiedOn,
	}
	require.NoError(t, repositories.NewPaymentIntentRepository(testContext.database).Save(t.Context(), intent))
	transaction, err := payment.NewTransaction(intent.ID, paymentaccount.PaymentProvider("mercado_pago"), payment.ExternalPayment{
		ID: uuid.NewString(), SellerAccountID: "test-seller-account-without-connection",
		ExternalReference: intent.ID, Status: payment.ExternalPaymentStatusApproved,
		Currency: "ARS", AmountCents: intent.TotalAmountCents,
	}, verifiedOn)
	require.NoError(t, err)
	require.NoError(t, repositories.NewPaymentTransactionRepository(testContext.database).Save(t.Context(), transaction))
	return int64(transaction.ID)
}

func TestProviderCollectionReaderHonorsSubmicrosecondHalfOpenBounds(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-nanosecond")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(48*time.Hour), uuid.NewString())
	setCollectionTerms(t, testContext, order.ServiceProposalID(), 100000, 20000, 5000, 1000)
	verified := base.Add(20 * time.Hour)
	saveCollectionPayment(t, testContext, order.ServiceProposalID(), "booking_deposit", 20000, 1000, verified)
	reader := repositories.NewProviderCollectionReader(testContext.database)
	for _, test := range []struct {
		name       string
		from, to   time.Time
		wantCount  int64
		wantAmount int64
	}{
		{"inside with fractional bounds", verified.Add(-time.Nanosecond), verified.Add(time.Nanosecond), 1, 20000},
		{"at excluded end", verified.Add(-time.Hour), verified, 0, 0},
		{"before fractional start", verified.Add(time.Nanosecond), verified.Add(time.Hour), 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			period := provider.ActivityQuery{From: test.from, To: test.to, Granularity: provider.ActivityDay}
			summary, err := reader.ReadSummary(t.Context(), fixture.providerID, period)
			require.NoError(t, err)
			require.Equal(t, test.wantAmount, summary.Current.BookingDepositCents)
			detail, err := reader.ReadDetail(t.Context(), fixture.providerID, provider.CollectionDetailQuery{Period: period, Limit: 20})
			require.NoError(t, err)
			require.Equal(t, test.wantCount, detail.TotalCount)
			require.Equal(t, test.wantAmount, detail.TotalAmountCents)
		})
	}
}

func TestProviderCollectionReaderRejectsDuplicateApprovedTransactionForAnIntent(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-duplicate")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(48*time.Hour), uuid.NewString())
	setCollectionTerms(t, testContext, order.ServiceProposalID(), 100000, 20000, 5000, 1000)
	verified := base.Add(20 * time.Hour)
	originalID := saveCollectionPayment(t, testContext, order.ServiceProposalID(), "booking_deposit", 20000, 1000, verified)
	_, err := testContext.database.ExecContext(t.Context(), `
		INSERT INTO payment_transactions (payment_intent_id, processor, external_payment_id, seller_account_id,
			status, currency, amount_cents, verified_on, created_on, updated_on)
		SELECT payment_intent_id, processor, $2, seller_account_id, status, currency,
			amount_cents, verified_on, created_on, updated_on
		FROM payment_transactions WHERE id = $1`, originalID, uuid.NewString())
	require.NoError(t, err)
	reader := repositories.NewProviderCollectionReader(testContext.database)
	period := provider.ActivityQuery{From: verified.Add(-time.Hour), To: verified.Add(time.Hour), Granularity: provider.ActivityDay}
	_, err = reader.ReadSummary(t.Context(), fixture.providerID, period)
	require.ErrorContains(t, err, "inconsistent")
	_, err = reader.ReadDetail(t.Context(), fixture.providerID, provider.CollectionDetailQuery{Period: period, Limit: 20})
	require.ErrorContains(t, err, "inconsistent")
}

func TestProviderCollectionReaderOrdersEqualVerificationInstantsByDescendingID(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "collection-tie")
	base := time.Now().UTC().Truncate(time.Microsecond)
	verified := base.Add(20 * time.Hour)
	var IDs []int64
	for index := 0; index < 2; index++ {
		order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(time.Duration(index+48)*time.Hour), uuid.NewString())
		setCollectionTerms(t, testContext, order.ServiceProposalID(), 100000, 20000, 5000, 1000)
		IDs = append(IDs, saveCollectionPayment(t, testContext, order.ServiceProposalID(), "booking_deposit", 20000, 1000, verified))
	}
	reader := repositories.NewProviderCollectionReader(testContext.database)
	period := provider.ActivityQuery{From: verified.Add(-time.Hour), To: verified.Add(time.Hour), Granularity: provider.ActivityDay}
	query := provider.CollectionDetailQuery{Period: period, Limit: 1}
	first, err := reader.ReadDetail(t.Context(), fixture.providerID, query)
	require.NoError(t, err)
	require.Len(t, first.Page, 1)
	require.Equal(t, IDs[1], first.Page[0].ID)
	require.NotNil(t, first.Next)
	query.After = first.Next
	second, err := reader.ReadDetail(t.Context(), fixture.providerID, query)
	require.NoError(t, err)
	require.Len(t, second.Page, 1)
	require.Equal(t, IDs[0], second.Page[0].ID)
	require.Nil(t, second.Next)
}
