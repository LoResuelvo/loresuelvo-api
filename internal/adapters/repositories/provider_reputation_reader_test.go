package repositories_test

import (
	"context"
	"net/url"
	"slices"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderReputationReaderIncludesOnlyPaidOrdersAndReviews(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-paid-only")
	base := time.Now().UTC().Truncate(time.Microsecond)
	paid := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString(), 4, "Paid order")
	savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(27*time.Hour), uuid.NewString())
	for index, status := range []string{"scheduled", "awaiting_payment"} {
		order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(time.Duration(index+28)*time.Hour), uuid.NewString(), 1, "Historical unpaid review")
		// Legacy reviews can remain attached to a nonpaid order. They are not
		// current eligible reputation facts, even though the review is persisted.
		_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET status = $1 WHERE id = $2`, status, order.ID())
		require.NoError(t, err)
	}

	snapshot, err := repositories.NewProviderReputationReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), snapshot.EligiblePaidOrders)
	require.Equal(t, int64(1), snapshot.ReviewedPaidOrders)
	require.Equal(t, [5]int64{0, 0, 0, 1, 0}, snapshot.RatingDistribution)
	require.Equal(t, []readmodel.ReputationReview{{WorkOrderID: paid.ID(), Rating: 4, Description: "Paid order"}}, snapshot.Reviews)
}

func TestProviderReputationReaderRetainsAllStarsAndStoredDescriptions(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-stars")
	base := time.Now().UTC().Truncate(time.Microsecond)
	var expected []readmodel.ReputationReview
	for rating := 1; rating <= 5; rating++ {
		description := "Stored description"
		if rating == 3 {
			description = ""
		}
		order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(time.Duration(rating+26)*time.Hour), uuid.NewString(), rating, description)
		expected = append(expected, readmodel.ReputationReview{WorkOrderID: order.ID(), Rating: rating, Description: description})
	}
	slices.Reverse(expected)

	snapshot, err := repositories.NewProviderReputationReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, [5]int64{1, 1, 1, 1, 1}, snapshot.RatingDistribution)
	require.Equal(t, expected, snapshot.Reviews)
	require.Nil(t, snapshot.Next)
}

func TestProviderReputationReaderIsolatesProviders(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-own")
	foreign := newProviderWorkOrderTestFixture(t, testContext, "reputation-foreign")
	base := time.Now().UTC().Truncate(time.Microsecond)
	own := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString(), 5, "Own review")
	savePaidWorkOrderWithReviewForFixture(t, testContext, foreign, base.Add(27*time.Hour), uuid.NewString(), 1, "Foreign review")
	savePaidWorkOrderWithoutReviewForFixture(t, testContext, foreign, base.Add(28*time.Hour), uuid.NewString())

	snapshot, err := repositories.NewProviderReputationReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), snapshot.EligiblePaidOrders)
	require.Equal(t, int64(1), snapshot.ReviewedPaidOrders)
	require.Equal(t, [5]int64{0, 0, 0, 0, 1}, snapshot.RatingDistribution)
	require.Equal(t, []readmodel.ReputationReview{{WorkOrderID: own.ID(), Rating: 5, Description: "Own review"}}, snapshot.Reviews)
}

func TestProviderReputationServiceDistinguishesNoOrdersFromPaidUnreviewedOrders(t *testing.T) {
	for _, paidCount := range []int{0, 2} {
		t.Run(map[int]string{0: "no orders", 2: "paid unreviewed"}[paidCount], func(t *testing.T) {
			testContext := newServiceProposalRepositoryTest(t)
			fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-empty")
			base := time.Now().UTC().Truncate(time.Microsecond)
			for index := 0; index < paidCount; index++ {
				savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(time.Duration(index+26)*time.Hour), uuid.NewString())
			}
			service := provider.NewReputationService(repositories.NewProviderReputationReader(testContext.database), repositories.NewProviderActivityActorFinder(testContext.database), clockadapter.NewSystemClock())
			result, id, err := service.Query(t.Context(), fixture.providerAuthID, provider.ReputationQueryInput{})
			require.NoError(t, err)
			require.Equal(t, fixture.providerID, id)
			require.Equal(t, int64(paidCount), result.EligiblePaidOrders)
			require.Zero(t, result.ReviewedPaidOrders)
			require.Equal(t, [5]int64{}, result.RatingDistribution)
			require.Nil(t, result.AverageRating)
			if paidCount == 0 {
				require.Nil(t, result.CoveragePercentage)
			} else {
				require.Equal(t, float64(0), *result.CoveragePercentage)
			}
			require.Empty(t, result.Reviews)
			require.NotNil(t, result.Reviews)
			require.Nil(t, result.Next)
		})
	}
}

func TestProviderReputationReaderDoesNotDuplicateReviewsAcrossImagesAndPaymentAttempts(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-fanout")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString(), 4, "One review")
	addActivityCompletionImages(t, testContext, fixture.providerAuthID, order.ID(), 2)
	for index := 0; index < 3; index++ {
		_, err := testContext.database.ExecContext(t.Context(), `INSERT INTO payment_intents
			(id, service_proposal_id, purpose, currency, seller_amount_cents, platform_fee_cents, total_amount_cents, status, created_on, updated_on)
			VALUES ($1, $2, 'service_balance', 'ARS', 100, 10, 110, 'rejected', $3, $3)`, uuid.NewString(), order.ServiceProposalID(), base)
		require.NoError(t, err)
	}
	var images, attempts int
	err := testContext.database.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM work_order_completion_images image
		JOIN work_order_completion_reports report ON report.id = image.completion_report_id WHERE report.work_order_id = $1`, order.ID()).Scan(&images)
	require.NoError(t, err)
	err = testContext.database.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM payment_intents WHERE service_proposal_id = $1`, order.ServiceProposalID()).Scan(&attempts)
	require.NoError(t, err)
	require.Equal(t, 3, images, "the fixture must actually exercise image multiplicity")
	require.Equal(t, 3, attempts, "the fixture must actually exercise payment-attempt multiplicity")

	snapshot, err := repositories.NewProviderReputationReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), snapshot.EligiblePaidOrders)
	require.Equal(t, int64(1), snapshot.ReviewedPaidOrders)
	require.Equal(t, [5]int64{0, 0, 0, 1, 0}, snapshot.RatingDistribution)
	require.Len(t, snapshot.Reviews, 1)
	require.Equal(t, order.ID(), snapshot.Reviews[0].WorkOrderID)
}

func TestProviderReputationReaderPaginatesAllReviewsWithGlobalTotalsAndMaximumLimit(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-pagination")
	base := time.Now().UTC().Truncate(time.Microsecond)
	var expectedIDs []int
	for index := 0; index < 105; index++ {
		order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(time.Duration(index+26)*time.Hour), uuid.NewString(), index%5+1, "")
		expectedIDs = append(expectedIDs, order.ID())
	}
	slices.Reverse(expectedIDs)
	reader := repositories.NewProviderReputationReader(testContext.database)
	query := provider.ReputationQuery{Limit: 20}
	var seen []int
	for page := 0; page < 6; page++ {
		snapshot, err := reader.Read(t.Context(), fixture.providerID, query)
		require.NoError(t, err)
		require.Equal(t, int64(105), snapshot.EligiblePaidOrders)
		require.Equal(t, int64(105), snapshot.ReviewedPaidOrders)
		require.Equal(t, [5]int64{21, 21, 21, 21, 21}, snapshot.RatingDistribution)
		for _, review := range snapshot.Reviews {
			seen = append(seen, review.WorkOrderID)
		}
		if page == 5 {
			require.Len(t, snapshot.Reviews, 5)
			require.Nil(t, snapshot.Next)
			break
		}
		require.Len(t, snapshot.Reviews, 20)
		require.NotNil(t, snapshot.Next)
		require.Equal(t, snapshot.Reviews[19].WorkOrderID, snapshot.Next.WorkOrderID)
		query.After = snapshot.Next
	}
	require.Equal(t, expectedIDs, seen, "every review appears once, in descending work-order ID order")
	maximum, err := reader.Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 100})
	require.NoError(t, err)
	require.Len(t, maximum.Reviews, 100)
	require.NotNil(t, maximum.Next)
	require.Equal(t, expectedIDs[99], maximum.Next.WorkOrderID)
	last, err := reader.Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 100, After: maximum.Next})
	require.NoError(t, err)
	require.Len(t, last.Reviews, 5)
	require.Nil(t, last.Next)
	pastLast, err := reader.Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 20, After: &readmodel.ReputationPosition{WorkOrderID: expectedIDs[len(expectedIDs)-1]}})
	require.NoError(t, err)
	require.Empty(t, pastLast.Reviews)
	require.Equal(t, int64(105), pastLast.ReviewedPaidOrders)
}

func TestProviderReputationReaderRejectsMissingProviderAndCancelledContext(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	reader := repositories.NewProviderReputationReader(testContext.database)
	snapshot, err := reader.Read(t.Context(), 2147483647, provider.ReputationQuery{Limit: 20})
	require.Nil(t, snapshot)
	require.ErrorIs(t, err, provider.ErrReputationProviderNotFound)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	snapshot, err = reader.Read(ctx, 2147483647, provider.ReputationQuery{Limit: 20})
	require.Nil(t, snapshot)
	require.ErrorIs(t, err, context.Canceled)
}

func TestProviderReputationReaderRejectsInvalidQueries(t *testing.T) {
	// Invalid inputs must be rejected without requiring a database connection.
	reader := repositories.NewProviderReputationReader(nil)
	for _, test := range []struct {
		id    int
		query provider.ReputationQuery
	}{
		{0, provider.ReputationQuery{Limit: 20}}, {7, provider.ReputationQuery{}},
		{7, provider.ReputationQuery{Limit: 101}}, {7, provider.ReputationQuery{Limit: 20, After: &readmodel.ReputationPosition{}}},
	} {
		snapshot, err := reader.Read(t.Context(), test.id, test.query)
		require.Nil(t, snapshot)
		require.ErrorIs(t, err, provider.ErrInvalidReputationQuery)
	}
}

func TestProviderReputationReaderDoesNotConvertSQLSourceFailureToEmptyActivity(t *testing.T) {
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	parsed, err := url.Parse(config.URL)
	require.NoError(t, err)
	values := parsed.Query()
	values.Set("search_path", "reputation_missing_test_schema")
	parsed.RawQuery = values.Encode()
	config.URL = parsed.String()
	database, err := db.ConnectPostgres(t.Context(), config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	snapshot, err := repositories.NewProviderReputationReader(database).Read(t.Context(), 7, provider.ReputationQuery{Limit: 20})
	require.Nil(t, snapshot)
	require.ErrorContains(t, err, "checking reputation provider")
}

func TestProviderReputationReaderKeepsRepeatableReadSnapshotAcrossTotalsAndPage(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "reputation-snapshot")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString(), 5, "Original review")
	writer, err := testContext.database.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	writerCommitted := false
	defer func() {
		if !writerCommitted {
			require.NoError(t, writer.Rollback())
		}
	}()
	_, err = writer.ExecContext(t.Context(), `LOCK TABLE work_order_reviews IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	type readResult struct {
		snapshot *readmodel.ReputationSnapshot
		err      error
	}
	results := make(chan readResult, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	go func() {
		snapshot, readErr := repositories.NewProviderReputationReader(testContext.database).Read(ctx, fixture.providerID, provider.ReputationQuery{Limit: 20})
		results <- readResult{snapshot: snapshot, err: readErr}
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var waiting bool
		err := testContext.database.QueryRowContext(t.Context(), `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity WHERE datname = current_database()
			AND state = 'active' AND wait_event_type = 'Lock'
			AND query LIKE '%COUNT(review.work_order_id)%'
		)`).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, blocked, "the reader must establish its actor snapshot and reach the locked aggregate query")
	_, err = writer.ExecContext(t.Context(), `UPDATE work_order_reviews SET rating = 1, description = 'Concurrent review' WHERE work_order_id = $1`, order.ID())
	require.NoError(t, err)
	require.NoError(t, writer.Commit())
	writerCommitted = true
	select {
	case result := <-results:
		require.NoError(t, result.err)
		require.Equal(t, [5]int64{0, 0, 0, 0, 1}, result.snapshot.RatingDistribution)
		require.Equal(t, []readmodel.ReputationReview{{WorkOrderID: order.ID(), Rating: 5, Description: "Original review"}}, result.snapshot.Reviews)
	case <-time.After(5 * time.Second):
		t.Fatal("reputation reader did not complete after releasing the writer lock")
	}
	fresh, err := repositories.NewProviderReputationReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ReputationQuery{Limit: 20})
	require.NoError(t, err)
	require.Equal(t, [5]int64{1, 0, 0, 0, 0}, fresh.RatingDistribution)
	require.Equal(t, "Concurrent review", fresh.Reviews[0].Description)
}
