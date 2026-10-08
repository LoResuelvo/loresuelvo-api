package repositories_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderDiagnosticActivitySuppressesHiddenOriginalWithoutInvalidatingRating(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "diagnostic-hidden-review")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString(), 5, "Restricted original")
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_order_reviews SET visible = FALSE WHERE work_order_id = $1`, order.ID())
	require.NoError(t, err)
	result, err := repositories.NewProviderDiagnosticReader(testContext.database).FindByProviderID(t.Context(), fixture.providerID)
	require.NoError(t, err)
	require.Empty(t, result.Reviews)
	require.EqualValues(t, 1, result.RatingStats.Count)
	require.EqualValues(t, 5, result.RatingStats.Total)
}
