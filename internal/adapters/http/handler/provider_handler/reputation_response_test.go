package provider_handler

import (
	"testing"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/stretchr/testify/require"
)

func TestReputationResponseSeparatesRegisteredRatingsFromVisibleReviews(t *testing.T) {
	response := reputationResponseFromDomain(&readmodel.Reputation{ReviewedPaidOrders: 5, VisibleReviews: 2, Reviews: []readmodel.ReputationReview{{WorkOrderID: 3, Rating: 4, Description: "Visible"}}})
	require.Equal(t, int64(5), response.ReviewCount)
	require.Equal(t, int64(5), response.ReviewedPaidOrders)
	require.Equal(t, int64(2), response.VisibleReviewCount)
	require.Len(t, response.Reviews, 1, "visible total is global, not the size of this page")
}
