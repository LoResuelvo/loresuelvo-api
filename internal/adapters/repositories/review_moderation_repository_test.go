package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/require"
)

func TestReviewModerationRepositorySurvivesStaleOrdinaryOrderSave(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	_, err := f.service.Moderate(t.Context(), f.operator.AuthID(), f.order.ID(), hideReviewInput(1), "hide-before-stale-save")
	require.NoError(t, err)
	require.True(t, f.order.Review().Visible(), "the ordinary save must actually carry a stale visible review")
	_, err = f.testContext.workOrderRepository.Save(t.Context(), f.order)
	require.NoError(t, err)
	restored, err := f.testContext.workOrderRepository.FindByID(t.Context(), f.order.ID())
	require.NoError(t, err)
	require.False(t, restored.Review().Visible())
	require.Equal(t, 2, restored.Review().Version())
	require.Positive(t, restored.Review().HidingDecisionID())
	require.Equal(t, "Original preserved", restored.Review().Description())
}

func TestReviewModerationRepositorySurvivesStaleOrderWithoutReview(t *testing.T) {
	f := newReviewTransactionFixture(t, false)
	current, err := f.testContext.workOrderRepository.FindByID(t.Context(), f.order.ID())
	require.NoError(t, err)
	reviewer, err := f.testContext.userRepository.FindByAuthID(f.provider.consumerAuthID)
	require.NoError(t, err)
	review, err := workorder.NewReview(5, "Original preserved")
	require.NoError(t, err)
	require.NoError(t, current.AddReview(reviewer.(*consumer.Consumer), review))
	_, err = f.testContext.workOrderRepository.Save(t.Context(), current)
	require.NoError(t, err)
	_, err = f.service.Moderate(t.Context(), f.operator.AuthID(), f.order.ID(), hideReviewInput(1), "hide-before-no-review-save")
	require.NoError(t, err)
	require.Nil(t, f.order.Review(), "the stale aggregate must have no review")
	_, err = f.testContext.workOrderRepository.Save(t.Context(), f.order)
	require.NoError(t, err)
	restored, err := f.testContext.workOrderRepository.FindByID(t.Context(), f.order.ID())
	require.NoError(t, err)
	require.NotNil(t, restored.Review())
	require.False(t, restored.Review().Visible())
	require.Equal(t, 2, restored.Review().Version())
}

func TestReviewModerationRepositoryConcurrentNewReviewSavesCreateOnlyOnce(t *testing.T) {
	f := newReviewTransactionFixture(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	reviewer, err := f.testContext.userRepository.FindByAuthID(f.provider.consumerAuthID)
	require.NoError(t, err)
	orders := make([]*workorder.WorkOrder, 2)
	for index := range orders {
		orders[index], err = f.testContext.workOrderRepository.FindByID(ctx, f.order.ID())
		require.NoError(t, err)
		review, err := workorder.NewReview(5, "Original preserved")
		require.NoError(t, err)
		require.NoError(t, orders[index].AddReview(reviewer.(*consumer.Consumer), review))
	}
	gate, err := f.testContext.database.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() {
		rollbackErr := gate.Rollback()
		if !errors.Is(rollbackErr, sql.ErrTxDone) {
			require.NoError(t, rollbackErr)
		}
	}()
	_, err = gate.ExecContext(ctx, `SELECT id FROM work_orders WHERE id=$1 FOR UPDATE`, f.order.ID())
	require.NoError(t, err)
	type outcome struct {
		input *workorder.WorkOrder
		order *workorder.WorkOrder
		err   error
	}
	results := make(chan outcome, 2)
	for _, order := range orders {
		go func() {
			saved, err := f.testContext.workOrderRepository.Save(ctx, order)
			results <- outcome{order, saved, err}
		}()
	}
	waitForReviewLock(t, f, "UPDATE work_orders", 2)
	require.NoError(t, gate.Commit())
	first, second := <-results, <-results
	if first.err != nil {
		first, second = second, first
	}
	require.NoError(t, first.err)
	require.ErrorIs(t, second.err, workorder.ErrReviewAlreadyExists)
	require.Positive(t, first.order.Review().ID())
	var count int
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_order_reviews WHERE work_order_id=$1`, f.order.ID()).Scan(&count))
	require.Equal(t, 1, count)
	require.Zero(t, second.input.Review().ID(), "a rolled-back insertion must not mark the review persisted")
}

func TestReviewModerationRepositoryFailedCommitLeavesNewReviewRetryable(t *testing.T) {
	f := newReviewTransactionFixture(t, false)
	reviewer, err := f.testContext.userRepository.FindByAuthID(f.provider.consumerAuthID)
	require.NoError(t, err)
	review, err := workorder.NewReview(5, "Original preserved")
	require.NoError(t, err)
	require.NoError(t, f.order.AddReview(reviewer.(*consumer.Consumer), review))
	// A deferred constraint forces failure at Commit after the review INSERT succeeded.
	_, err = f.testContext.database.ExecContext(t.Context(), `CREATE FUNCTION us69_fail_review_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test deferred review failure'; END $$`)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := f.testContext.database.ExecContext(ctx, `DROP TRIGGER IF EXISTS us69_fail_review_commit ON work_order_reviews; DROP FUNCTION IF EXISTS us69_fail_review_commit()`)
		require.NoError(t, err)
	})
	_, err = f.testContext.database.ExecContext(t.Context(), `CREATE CONSTRAINT TRIGGER us69_fail_review_commit AFTER INSERT ON work_order_reviews DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION us69_fail_review_commit()`)
	require.NoError(t, err)
	_, err = f.testContext.workOrderRepository.Save(t.Context(), f.order)
	require.ErrorContains(t, err, "committing work order transaction")
	require.Zero(t, review.ID())
	var count int
	require.NoError(t, f.testContext.database.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM work_order_reviews WHERE work_order_id=$1`, f.order.ID()).Scan(&count))
	require.Zero(t, count)
	_, err = f.testContext.database.ExecContext(t.Context(), `DROP TRIGGER us69_fail_review_commit ON work_order_reviews`)
	require.NoError(t, err)
	_, err = f.testContext.workOrderRepository.Save(t.Context(), f.order)
	require.NoError(t, err)
	require.Equal(t, f.order.ID(), review.ID())
}
