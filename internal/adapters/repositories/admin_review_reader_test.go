package repositories_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdminReviewReaderUsesCanonicalFirstAndLaterProposalIdentities(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	var requestID int
	err := f.testContext.database.QueryRow(`INSERT INTO job_requests(consumer_id,provider_id,conversation_id,title,description,status,created_on) VALUES($1,$2,$3,'Actual request','Service request','accepted',$4) RETURNING id`, f.provider.consumerID, f.provider.providerID, f.provider.conversation.ID(), time.Now().UTC().Add(-time.Hour)).Scan(&requestID)
	require.NoError(t, err)
	later := savePaidWorkOrderWithReviewForFixture(t, f.testContext, f.provider, time.Now().UTC().Add(72*time.Hour), uuid.NewString(), 3, "Later review")
	reader := repositories.NewAdminReviewReader(f.testContext.database, repositories.NewReviewModerationRepository(), f.reports)
	page, err := reader.FindPage(t.Context(), workorder.ReviewListInput{Status: "all"})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Len(t, page.Items, 2)
	require.Equal(t, later.ID(), page.Items[0].WorkOrderID)
	require.Equal(t, fmt.Sprintf("sp-%d", later.ServiceProposalID()), page.Items[0].OperationID)
	require.Equal(t, f.order.ID(), page.Items[1].WorkOrderID)
	require.Equal(t, fmt.Sprintf("jr-%d", requestID), page.Items[1].OperationID)
}
func TestAdminReviewReaderBoundsHistoryWithoutMultiplyingListRows(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	_, err := f.service.Moderate(t.Context(), f.operator.AuthID(), f.order.ID(), hideReviewInput(1), "hide")
	require.NoError(t, err)
	_, err = f.service.Moderate(t.Context(), f.operator.AuthID(), f.order.ID(), workorder.ModerationInput{Action: "unhide", ExpectedVersion: 2, Reason: "Review complete"}, "unhide")
	require.NoError(t, err)
	reader := repositories.NewAdminReviewReader(f.testContext.database, repositories.NewReviewModerationRepository(), f.reports)
	page, err := reader.FindPage(t.Context(), workorder.ReviewListInput{Status: "visible"})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	detail, err := reader.FindByID(t.Context(), f.order.ID(), workorder.ReviewPageInput{Limit: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, detail.Total)
	require.Len(t, detail.Decisions, 1)
	require.Equal(t, "unhide", detail.Decisions[0].Action())
	require.Equal(t, 3, detail.Review.Version())
}
