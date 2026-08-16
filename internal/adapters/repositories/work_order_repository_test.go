package repositories_test

import (
	"context"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkOrderRepositoryFindsOnlyOrdersScheduledInsideWindow(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		userRepository: testContext.userRepository,
	}, "auth0|urgent-consumer", "urgent.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|urgent-provider", "urgent.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(context.Background(), activeConversation)
	require.NoError(t, err)

	from := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	insideOrder := saveScheduledWorkOrderAt(t, testContext, activeConversation, consumerID, providerID, from.Add(time.Hour))
	saveScheduledWorkOrderAt(t, testContext, activeConversation, consumerID, providerID, from.Add(-time.Hour))
	saveScheduledWorkOrderAt(t, testContext, activeConversation, consumerID, providerID, from.Add(24*time.Hour))

	orders, err := testContext.workOrderRepository.FindScheduledBetween(t.Context(), from, from.Add(24*time.Hour))

	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, insideOrder.ID(), orders[0].ID())
	assert.Equal(t, consumerID, orders[0].ConsumerID())
	assert.Equal(t, providerID, orders[0].ProviderID())
	assert.Equal(t, int64(1500050), orders[0].Amount())
	proposal, ok := orders[0].ServiceProposal().(*serviceproposal.ServiceProposal)
	require.True(t, ok)
	assertBookingTermsEqual(t, bookingTermsForAmount(t, 1500050, from.Add(time.Hour)), proposal.BookingTerms)
	assert.Equal(t, from.Add(time.Hour), orders[0].ScheduledOn().UTC())

	foundByID, err := testContext.workOrderRepository.FindByID(t.Context(), insideOrder.ID())
	require.NoError(t, err)
	assert.Equal(t, insideOrder.ID(), foundByID.ID())
	assert.Equal(t, insideOrder.ServiceProposalID(), foundByID.ServiceProposalID())
	assert.Equal(t, consumerID, foundByID.ConsumerID())
	assert.Equal(t, providerID, foundByID.ProviderID())
	assert.Equal(t, insideOrder.RemainingAmountDue(), foundByID.RemainingAmountDue())

	completionImageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	_, err = testContext.database.Exec(
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, 'private', 'trabajo.jpg', 'image/jpeg', 1024, 'confirmed', 'private', 'work_order_completion_image', 'auth0|urgent-provider', NOW(), NOW())`,
		completionImageID,
		"files/2026/08/work_order_completion_image/"+completionImageID+"/trabajo.jpg",
	)
	require.NoError(t, err)
	report, err := workorder.NewCompletionReport(
		"Trabajo finalizado y funcionamiento verificado.",
		[]string{completionImageID},
		insideOrder.ScheduledOn(),
	)
	require.NoError(t, err)
	require.NoError(t, insideOrder.ReportCompletion(providerID, report))
	updated, err := testContext.workOrderRepository.Save(t.Context(), insideOrder)
	require.NoError(t, err)
	assert.Equal(t, insideOrder.ID(), updated.ID())
	assert.Equal(t, workorder.StatusAwaitingPayment, updated.Status())
	assert.Equal(t, []string{completionImageID}, updated.CompletionReport().ImageFileIDs())

	paidOn := insideOrder.ScheduledOn().Add(time.Hour)
	require.NoError(t, updated.RegisterApprovedBalancePayment(paidOn))
	_, err = testContext.workOrderRepository.Save(t.Context(), updated)
	require.NoError(t, err)

	fullyPaid, err := testContext.workOrderRepository.FindByID(t.Context(), insideOrder.ID())
	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, fullyPaid.Status())
	assert.Equal(t, paidOn, fullyPaid.PaidOn())
	assert.Equal(t, report.Description(), fullyPaid.CompletionReport().Description())
	assert.Equal(t, []string{completionImageID}, fullyPaid.CompletionReport().ImageFileIDs())
}

func saveScheduledWorkOrderAt(
	t *testing.T,
	testContext serviceProposalRepositoryTestContext,
	activeConversation conversation.Conversation,
	consumerID int,
	providerID int,
	scheduledOn time.Time,
) *workorder.WorkOrder {
	t.Helper()
	proposal, err := serviceproposal.NewServiceProposal(
		&provider.Provider{BaseUser: user.RehydrateBaseUser(providerID, "", "", "", "", "", nil)},
		&consumer.Consumer{BaseUser: user.RehydrateBaseUser(consumerID, "", "", "", "", "", nil)},
		activeConversation,
		scheduledOn,
		"Urgent work order repository test.",
		bookingTermsForAmount(t, 1500050, scheduledOn),
		clockadapter.NewSystemClock(),
	)
	require.NoError(t, err)
	proposal, err = testContext.serviceProposalRepository.Save(proposal)
	require.NoError(t, err)
	proposal.Status = serviceproposal.StatusAccepted
	order, err := workorder.New(proposal, time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, err)
	savedOrder, err := testContext.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	return savedOrder
}
