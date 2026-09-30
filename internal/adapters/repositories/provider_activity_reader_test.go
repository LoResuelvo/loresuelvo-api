package repositories_test

import (
	"math"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderActivityReaderAggregatesIndependentMilestonesAndPreviousPeriod(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "activity-reader-periods")
	base := time.Now().UTC().Truncate(time.Microsecond)
	previousOrder := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString())
	currentOrder := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(27*time.Hour), uuid.NewString(), 5, "Persisted review")
	addActivityCompletionImages(t, testContext, fixture.providerAuthID, currentOrder.ID(), 2)
	setActivityOrderTimes(t, testContext, previousOrder.ID(), previousOrder.ServiceProposalID(), base.Add(-72*time.Hour), base.Add(-71*time.Hour), base.Add(-70*time.Hour), 500)
	setActivityOrderTimes(t, testContext, currentOrder.ID(), currentOrder.ServiceProposalID(), base.Add(-12*time.Hour), base.Add(-8*time.Hour), base.Add(-4*time.Hour), 10001)

	query := provider.ActivityQuery{
		From: base.Add(-48 * time.Hour), To: base,
		Granularity: provider.ActivityDay, ComparePrevious: true,
	}
	snapshot, err := repositories.NewProviderActivityReader(testContext.database).Read(t.Context(), fixture.providerID, query)

	require.NoError(t, err)
	require.Equal(t, int64(1), snapshot.Current.Confirmed)
	require.Equal(t, int64(1), snapshot.Current.Reported)
	require.Equal(t, int64(1), snapshot.Current.Paid)
	require.Equal(t, int64(1), snapshot.Current.Customers)
	require.Equal(t, int64(0), snapshot.Current.NewCustomers)
	require.Equal(t, int64(1), snapshot.Current.ReturningCustomers)
	require.Equal(t, int64(10001), snapshot.Current.ContractValueCents)
	require.NotNil(t, snapshot.Previous)
	require.Equal(t, int64(1), snapshot.Previous.Reported)
	require.Equal(t, int64(500), snapshot.Previous.ContractValueCents)
	require.NotEmpty(t, snapshot.Series)
	var bucketConfirmed, bucketReported, bucketPaid int64
	for _, bucket := range snapshot.Series {
		bucketConfirmed += bucket.Confirmed
		bucketReported += bucket.Reported
		bucketPaid += bucket.Paid
	}
	require.Equal(t, snapshot.Current.Confirmed, bucketConfirmed)
	require.Equal(t, snapshot.Current.Reported, bucketReported)
	require.Equal(t, snapshot.Current.Paid, bucketPaid)
}

func TestProviderActivityReaderRejectsContractValueOverflow(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "activity-reader-overflow")
	base := time.Now().UTC().Truncate(time.Microsecond)
	for index := 0; index < 2; index++ {
		order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(time.Duration(index+26)*time.Hour), uuid.NewString())
		setActivityOrderTimes(t, testContext, order.ID(), order.ServiceProposalID(), base.Add(time.Duration(index)*time.Hour), base.Add(time.Duration(index)*time.Hour), base.Add(time.Duration(index)*time.Hour), math.MaxInt64)
	}

	_, err := repositories.NewProviderActivityReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ActivityQuery{
		From: base.Add(-time.Minute), To: base.Add(3 * time.Hour), Granularity: provider.ActivityDay,
	})
	require.ErrorContains(t, err, "outside the supported integer range")
}

func TestProviderActivityReaderRoundsSubMicrosecondBoundsForHalfOpenPeriods(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "activity-reader-microsecond-bounds")
	instant := time.Now().UTC().Truncate(time.Microsecond)
	for index, occurredOn := range []time.Time{instant, instant.Add(time.Microsecond)} {
		order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, instant.Add(time.Duration(index+26)*time.Hour), uuid.NewString())
		setActivityOrderTimes(t, testContext, order.ID(), order.ServiceProposalID(), occurredOn, occurredOn, occurredOn, 10001)
	}
	reader := repositories.NewProviderActivityReader(testContext.database)

	fromAfterFirstInstant, err := reader.Read(t.Context(), fixture.providerID, provider.ActivityQuery{
		From: instant.Add(time.Nanosecond), To: instant.Add(2 * time.Microsecond), Granularity: provider.ActivityDay,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), fromAfterFirstInstant.Current.Confirmed)
	require.Equal(t, int64(1), fromAfterFirstInstant.Current.Reported)
	require.Equal(t, int64(1), fromAfterFirstInstant.Current.Paid)
	require.Equal(t, int64(10001), fromAfterFirstInstant.Current.ContractValueCents)
	require.Nil(t, fromAfterFirstInstant.Previous, "comparison remains absent unless explicitly requested")

	toBeforeCeiledInstant, err := reader.Read(t.Context(), fixture.providerID, provider.ActivityQuery{
		From: instant.Add(-time.Microsecond), To: instant.Add(time.Nanosecond), Granularity: provider.ActivityDay,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), toBeforeCeiledInstant.Current.Confirmed)
	require.Equal(t, int64(1), toBeforeCeiledInstant.Current.Reported)
	require.Equal(t, int64(1), toBeforeCeiledInstant.Current.Paid)
	require.Equal(t, int64(10001), toBeforeCeiledInstant.Current.ContractValueCents)
	require.Nil(t, toBeforeCeiledInstant.Previous)
}

func TestProviderActivityReaderDoesNotCountAwaitingPaymentWithoutReportedCompletion(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "activity-reader-inconsistent-pending")
	now := time.Now().UTC().Truncate(time.Microsecond)
	order := saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, now.Add(26*time.Hour))
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET status = 'awaiting_payment' WHERE id = $1`, order.ID())
	require.NoError(t, err)

	snapshot, err := repositories.NewProviderActivityReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ActivityQuery{
		From: now.Add(-time.Minute), To: now, Granularity: provider.ActivityDay,
	})
	require.NoError(t, err)
	require.Zero(t, snapshot.Pending.AwaitingPayment)
}

func TestProviderActivityReaderKeepsRepeatableReadSnapshotAcrossAggregates(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "activity-reader-snapshot")
	now := time.Now().UTC().Truncate(time.Microsecond)
	order := saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, now.Add(26*time.Hour))
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET accepted_on = $1 WHERE id = $2`, now.Add(-24*time.Hour), order.ID())
	require.NoError(t, err)
	writer, err := testContext.database.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(t.Context(), `LOCK TABLE work_orders IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)

	reader := repositories.NewProviderActivityReader(testContext.database)
	type readResult struct {
		snapshot *readmodel.ActivitySnapshot
		err      error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		snapshot, readErr := reader.Read(t.Context(), fixture.providerID, provider.ActivityQuery{
			From: now.Add(-time.Hour), To: now, Granularity: provider.ActivityDay,
		})
		resultCh <- readResult{snapshot: snapshot, err: readErr}
	}()

	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var waiting bool
		err := testContext.database.QueryRowContext(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname = current_database() AND state = 'active'
			  AND wait_event_type = 'Lock' AND query LIKE '%WITH first_completion_by_consumer AS (%'
			)`).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		_ = writer.Rollback()
		t.Fatal("reader did not reach the aggregate query while the writer held the table lock")
	}
	_, err = writer.ExecContext(t.Context(), `UPDATE work_orders SET accepted_on = $1 WHERE id = $2`, now.Add(-time.Minute), order.ID())
	require.NoError(t, err)
	require.NoError(t, writer.Commit())

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Zero(t, result.snapshot.Current.Confirmed, "the reader must retain the snapshot established before the concurrent commit")
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not finish after the writer committed")
	}
}

func setActivityOrderTimes(t *testing.T, testContext serviceProposalRepositoryTestContext, orderID, proposalID int, acceptedOn, reportedOn, paidOn time.Time, amountCents int64) {
	t.Helper()
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET accepted_on = $1, paid_on = $2 WHERE id = $3`, acceptedOn, paidOn, orderID)
	require.NoError(t, err)
	_, err = testContext.database.ExecContext(t.Context(), `UPDATE work_order_completion_reports SET reported_on = $1 WHERE work_order_id = $2`, reportedOn, orderID)
	require.NoError(t, err)
	_, err = testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET amount_cents = $1, deposit_cents = 1 WHERE id = $2`, amountCents, proposalID)
	require.NoError(t, err)
}

func addActivityCompletionImages(t *testing.T, testContext serviceProposalRepositoryTestContext, providerAuthID string, orderID, additionalImages int) {
	t.Helper()
	var reportID int
	err := testContext.database.QueryRowContext(t.Context(), `SELECT id FROM work_order_completion_reports WHERE work_order_id = $1`, orderID).Scan(&reportID)
	require.NoError(t, err)
	for position := 1; position <= additionalImages; position++ {
		fileID := uuid.NewString()
		saveReviewCompletionImage(t, testContext, providerAuthID, fileID)
		_, err := testContext.database.ExecContext(t.Context(), `INSERT INTO work_order_completion_images(completion_report_id, file_id, position) VALUES ($1, $2, $3)`, reportID, fileID, position)
		require.NoError(t, err)
	}
}
