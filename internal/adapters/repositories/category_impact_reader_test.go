package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/require"
)

func TestCategoryImpactReaderReturnsMissingCategory(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	impact, err := repositories.NewCategoryImpactReader(fixture.testContext.database).FindByCategoryID(t.Context(), 999999999, time.Now().UTC())
	require.ErrorIs(t, err, category.ErrDoesNotExist)
	require.Nil(t, impact)
}
func TestCategoryImpactReaderPropagatesCancelledContext(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	impact, err := repositories.NewCategoryImpactReader(fixture.testContext.database).FindByCategoryID(ctx, 123, time.Now().UTC())
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, impact)
}
func TestCategoryImpactReaderCountsIdentitiesAndPendingProposalTimeBoundary(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	at := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	providerID := savedProviderIDForJobRequest(t, fixture.testContext)
	provider, err := fixture.testContext.userRepository.FindProviderByID(t.Context(), providerID)
	require.NoError(t, err)
	reader := repositories.NewCategoryImpactReader(fixture.testContext.database)
	pendingConsumer := savedConsumerIDWithData(t, fixture.testContext, "auth0|impact-pending", "impact.pending@example.com", "Ana", "Pending")
	fixture.jobRequest(t, pendingConsumer, providerID, at.Add(-time.Hour), "pending")
	unproposedConsumer := savedConsumerIDWithData(t, fixture.testContext, "auth0|impact-unproposed", "impact.unproposed@example.com", "Ana", "Unproposed")
	fixture.jobRequest(t, unproposedConsumer, providerID, at.Add(-time.Hour), "accepted")
	consumer := savedConsumerIDWithData(t, fixture.testContext, "auth0|impact-proposed", "impact.proposed@example.com", "Ana", "Proposed")
	request := fixture.jobRequest(t, consumer, providerID, at.Add(-time.Hour), "accepted")
	// Existing proposals of any status exclude an accepted request from the unproposed count.
	fixture.scheduledProposal(t, request, at.Add(-96*time.Hour), at.Add(-time.Hour), 60, "rejected")
	for _, scheduled := range []time.Time{at.Add(-time.Second), at, at.Add(time.Second), at.Add(time.Hour)} {
		fixture.scheduledProposal(t, request, at.Add(-96*time.Hour), scheduled, 60, "pending")
	}
	// Ongoing orders remain relevant regardless of scheduled date, and awaiting payment
	// does not require completion evidence to contribute to this operational snapshot.
	proposal := fixture.scheduledProposal(t, request, at.Add(-96*time.Hour), at.Add(-time.Hour), 60, "accepted")
	fixture.workOrder(t, proposal, at.Add(-72*time.Hour), "scheduled")
	proposal = fixture.scheduledProposal(t, request, at.Add(-96*time.Hour), at.Add(-time.Hour), 60, "accepted")
	fixture.workOrder(t, proposal, at.Add(-72*time.Hour), "awaiting_payment")
	proposal = fixture.scheduledProposal(t, request, at.Add(-96*time.Hour), at.Add(time.Hour), 60, "accepted")
	fixture.workOrder(t, proposal, at.Add(-72*time.Hour), "paid")
	otherProvider := savedProviderIDWithData(t, fixture.testContext, "auth0|impact-other", "impact.other@example.com", "Other", "Provider", "Electricidad")
	otherRequest := fixture.jobRequest(t, pendingConsumer, otherProvider, at.Add(-time.Hour), "accepted")
	otherProposal := fixture.scheduledProposal(t, otherRequest, at.Add(-96*time.Hour), at.Add(time.Hour), 60, "accepted")
	fixture.workOrder(t, otherProposal, at.Add(-72*time.Hour), "scheduled")
	impact, err := reader.FindByCategoryID(t.Context(), provider.Category.ID, at)
	require.NoError(t, err)
	require.Equal(t, category.ImpactCounts{AssignedProviders: 1, PendingRequests: 1, AcceptedRequestsWithoutProposal: 1, PendingProposals: 2, ScheduledOrders: 1, AwaitingPaymentOrders: 1}, impact.Counts)
	require.Equal(t, at, impact.ObservedAt)
}
func TestCategoryImpactReaderReturnsDisabledCategoryWithoutActivity(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	value, err := category.New("Electricidad")
	require.NoError(t, err)
	value.Enabled = false
	saved, err := fixture.testContext.categoryRepository.Save(*value)
	require.NoError(t, err)
	at := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	impact, err := repositories.NewCategoryImpactReader(fixture.testContext.database).FindByCategoryID(t.Context(), saved.ID, at)
	require.NoError(t, err)
	require.Equal(t, *saved, impact.Category)
	require.Equal(t, category.ImpactCounts{}, impact.Counts)
	require.False(t, impact.HasOngoingOrders())
	require.False(t, impact.RequiresConfirmation())
}
