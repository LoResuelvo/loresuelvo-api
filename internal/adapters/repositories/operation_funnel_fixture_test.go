package repositories_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/stretchr/testify/require"
)

func TestOperationFunnelReportFixtureRemoveCompletionReportPreservesPaidOn(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	base := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, base, "pending")
	proposalID := fixture.proposal(t, request, base.Add(time.Minute), "accepted")
	paidOrderID := fixture.workOrder(t, proposalID, base.Add(2*time.Minute), "paid")
	siblingProposalID := fixture.proposal(t, request, base.Add(3*time.Minute), "accepted")
	siblingOrderID := fixture.workOrder(t, siblingProposalID, base.Add(4*time.Minute), "paid")
	fixture.completionReport(t, paidOrderID, base.Add(5*time.Minute))
	fixture.completionReport(t, siblingOrderID, base.Add(6*time.Minute))
	paidOn := base.Add(7 * time.Minute)
	_, err := fixture.testContext.database.Exec(`UPDATE work_orders SET paid_on=$1 WHERE id=$2`, paidOn, paidOrderID)
	require.NoError(t, err)
	_, err = fixture.testContext.database.Exec(`UPDATE work_orders SET paid_on=$1 WHERE id=$2`, paidOn.Add(time.Minute), siblingOrderID)
	require.NoError(t, err)

	err = (testsupport.OperationDetailReportFixture{DB: fixture.testContext.database}).RemoveCompletionReport(t.Context(), paidOrderID)
	require.NoError(t, err)

	var targetReportCount, siblingReportCount int
	var targetPaidOn time.Time
	require.NoError(t, fixture.testContext.database.QueryRow(`SELECT COUNT(*) FROM work_order_completion_reports WHERE work_order_id=$1`, paidOrderID).Scan(&targetReportCount))
	require.NoError(t, fixture.testContext.database.QueryRow(`SELECT COUNT(*) FROM work_order_completion_reports WHERE work_order_id=$1`, siblingOrderID).Scan(&siblingReportCount))
	require.NoError(t, fixture.testContext.database.QueryRow(`SELECT paid_on FROM work_orders WHERE id=$1`, paidOrderID).Scan(&targetPaidOn))
	require.Zero(t, targetReportCount)
	require.Equal(t, 1, siblingReportCount)
	require.True(t, targetPaidOn.Equal(paidOn), "removing historical completion evidence must preserve paid_on")
}
