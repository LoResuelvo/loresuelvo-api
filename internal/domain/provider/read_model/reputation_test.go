package readmodel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReputationIncludesHiddenRatingsWithoutRequiringHiddenReviewsInPage(t *testing.T) {
	snapshot := ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, VisibleReviews: 1, RatingDistribution: [5]int64{0, 0, 1, 0, 1}, Reviews: []ReputationReview{{WorkOrderID: 7, Rating: 3, Description: "Visible"}}}
	result, err := snapshot.Calculate(time.Now(), 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.VisibleReviews)
	require.Equal(t, float64(4), *result.AverageRating)
	require.Equal(t, float64(100), *result.CoveragePercentage)
	require.Nil(t, result.Next)
}

func TestReputationAllHiddenRatingsHaveEmptyVisiblePage(t *testing.T) {
	snapshot := ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, RatingDistribution: [5]int64{0, 0, 1, 0, 1}}
	result, err := snapshot.Calculate(time.Now(), 1, 0)
	require.NoError(t, err)
	require.Empty(t, result.Reviews)
	require.Nil(t, result.Next)
	require.Equal(t, float64(4), *result.AverageRating)
}

func TestReputationContinuationUsesVisibleTotal(t *testing.T) {
	snapshot := ReputationSnapshot{EligiblePaidOrders: 5, ReviewedPaidOrders: 5, VisibleReviews: 2, RatingDistribution: [5]int64{5}, Reviews: []ReputationReview{{WorkOrderID: 7, Rating: 1}}}
	_, err := snapshot.Calculate(time.Now(), 1, 0)
	require.ErrorContains(t, err, "missing its continuation")
	snapshot.Next = &ReputationPosition{WorkOrderID: 7}
	result, err := snapshot.Calculate(time.Now(), 1, 0)
	require.NoError(t, err)
	require.NotNil(t, result.Next)
	snapshot.VisibleReviews = 1
	_, err = snapshot.Calculate(time.Now(), 1, 0)
	require.ErrorContains(t, err, "invalid continuation")
}

func TestReputationRejectsInvalidVisibleTotals(t *testing.T) {
	for _, count := range []int64{-1, 2} {
		snapshot := ReputationSnapshot{EligiblePaidOrders: 1, ReviewedPaidOrders: 1, VisibleReviews: count, RatingDistribution: [5]int64{1}}
		_, err := snapshot.Calculate(time.Now(), 20, 0)
		require.Error(t, err)
	}
}
