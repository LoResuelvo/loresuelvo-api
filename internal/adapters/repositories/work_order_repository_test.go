package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkOrderRepositoryStoresReview(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	order, review := savePaidWorkOrderWithReview(t, testContext)

	_, err := testContext.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)

	var storedRating int
	var storedDescription string
	require.NoError(t, testContext.database.QueryRow(
		`SELECT rating, description
		FROM work_order_reviews
		WHERE work_order_id = $1`,
		order.ID(),
	).Scan(&storedRating, &storedDescription))
	require.Equal(t, review.Rating(), storedRating)
	require.Equal(t, review.Description(), storedDescription)
}

func TestWorkOrderRepositoryHydratesReview(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	order, expectedReview := savePaidWorkOrderWithReview(t, testContext)
	_, err := testContext.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)

	found, err := testContext.workOrderRepository.FindByID(t.Context(), order.ID())
	require.NoError(t, err)
	require.Equal(t, expectedReview, found.Review())
}

func TestWorkOrderRepositoryFindsZeroProviderRatingStatsWithoutReviews(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "zero-rating")

	stats, err := testContext.workOrderRepository.FindRatingStatsByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	assert.Equal(t, provider.RatingStats{}, stats)
}

func TestWorkOrderRepositoryAggregatesProviderRatingStats(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "rating-stats")
	baseScheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, baseScheduledOn, "11111111-1111-1111-1111-111111111111", 5, "Muy buen trabajo.")
	savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, baseScheduledOn.Add(24*time.Hour), "22222222-2222-2222-2222-222222222222", 3, "Trabajo correcto.")

	stats, err := testContext.workOrderRepository.FindRatingStatsByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	assert.Equal(t, provider.RatingStats{Total: 8, Count: 2, Distribution: provider.RatingDistribution{0, 0, 1, 0, 1}}, stats)
}

func TestWorkOrderRepositoryAggregatesRatingStatsForMultipleProviders(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	firstFixture := newProviderWorkOrderTestFixture(t, testContext, "rating-stats-batch-first")
	secondFixture := newProviderWorkOrderTestFixture(t, testContext, "rating-stats-batch-second")
	baseScheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	savePaidWorkOrderWithReviewForFixture(t, testContext, firstFixture, baseScheduledOn, "99999999-9999-9999-9999-999999999991", 5, "Muy buen trabajo.")
	savePaidWorkOrderWithReviewForFixture(t, testContext, firstFixture, baseScheduledOn.Add(24*time.Hour), "99999999-9999-9999-9999-999999999992", 3, "Trabajo correcto.")
	savePaidWorkOrderWithReviewForFixture(t, testContext, secondFixture, baseScheduledOn, "99999999-9999-9999-9999-999999999993", 2, "Debe mejorar.")

	statsByProviderID, err := testContext.workOrderRepository.FindRatingStatsByProviderIDs(
		t.Context(),
		[]int{firstFixture.providerID, secondFixture.providerID},
	)

	require.NoError(t, err)
	assert.Equal(t, map[int]provider.RatingStats{
		firstFixture.providerID:  {Total: 8, Count: 2, Distribution: provider.RatingDistribution{0, 0, 1, 0, 1}},
		secondFixture.providerID: {Total: 2, Count: 1, Distribution: provider.RatingDistribution{0, 1, 0, 0, 0}},
	}, statsByProviderID)
}

func TestWorkOrderRepositoryOmitsProvidersWithoutReviewsFromRatingStatsBatch(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "rating-stats-batch-empty")

	statsByProviderID, err := testContext.workOrderRepository.FindRatingStatsByProviderIDs(t.Context(), []int{fixture.providerID})

	require.NoError(t, err)
	assert.Empty(t, statsByProviderID)
}

func TestWorkOrderRepositoryKeepsPaidWorkOrderWithoutReviewInProviderHistory(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "history-without-review")
	savePaidWorkOrderWithoutReviewForFixture(
		t,
		testContext,
		fixture,
		time.Now().UTC().Truncate(time.Microsecond).Add(48*time.Hour),
		"33333333-3333-3333-3333-333333333333",
	)

	history, err := testContext.workOrderRepository.FindPaidWorkHistoryByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Nil(t, history[0].Review)
}

func TestWorkOrderRepositoryIncludesReviewInProviderHistory(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "history-with-review")
	savePaidWorkOrderWithReviewForFixture(
		t,
		testContext,
		fixture,
		time.Now().UTC().Truncate(time.Microsecond).Add(48*time.Hour),
		"44444444-4444-4444-4444-444444444444",
		4,
		"Trabajo claro y prolijo.",
	)

	history, err := testContext.workOrderRepository.FindPaidWorkHistoryByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	require.Len(t, history, 1)
	require.NotNil(t, history[0].Review)
	assert.Equal(t, providerreadmodel.Review{Rating: 4, Description: "Trabajo claro y prolijo."}, *history[0].Review)
}

func TestWorkOrderRepositoryIncludesCompletionReportInProviderHistory(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "history-with-report")
	scheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, scheduledOn, "55555555-5555-5555-5555-555555555555")

	history, err := testContext.workOrderRepository.FindPaidWorkHistoryByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	require.Len(t, history, 1)
	require.NotNil(t, history[0].CompletionReport)
	assert.Equal(t, providerreadmodel.CompletionReport{
		Description: "Trabajo terminado",
		ReportedOn:  scheduledOn,
	}, *history[0].CompletionReport)
}

func TestWorkOrderRepositoryFiltersProviderHistoryToPaidOrders(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "history-paid-only")
	baseScheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	paidOrder := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, baseScheduledOn, "66666666-6666-6666-6666-666666666666")
	saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, baseScheduledOn.Add(24*time.Hour))

	history, err := testContext.workOrderRepository.FindPaidWorkHistoryByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, paidOrder.ID(), history[0].ID)
}

func TestWorkOrderRepositoryOrdersProviderHistoryByMostRecentScheduledDate(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "history-order")
	baseScheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	olderOrder := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, baseScheduledOn, "77777777-7777-7777-7777-777777777777")
	recentOrder := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, baseScheduledOn.Add(24*time.Hour), "88888888-8888-8888-8888-888888888888")

	history, err := testContext.workOrderRepository.FindPaidWorkHistoryByProviderID(t.Context(), fixture.providerID)

	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, []int{recentOrder.ID(), olderOrder.ID()}, []int{history[0].ID, history[1].ID})
}

func TestWorkOrderRepositoryFindsPaidHistoryForMultipleProvidersInOneResult(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	firstFixture := newProviderWorkOrderTestFixture(t, testContext, "history-batch-first")
	secondFixture := newProviderWorkOrderTestFixture(t, testContext, "history-batch-second")
	thirdFixture := newProviderWorkOrderTestFixture(t, testContext, "history-batch-empty")
	baseScheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	firstOrder := savePaidWorkOrderWithoutReviewForFixture(t, testContext, firstFixture, baseScheduledOn, "99999999-9999-9999-9999-999999999994")
	secondOrder := savePaidWorkOrderWithoutReviewForFixture(t, testContext, secondFixture, baseScheduledOn, "99999999-9999-9999-9999-999999999995")

	historyByProviderID, err := testContext.workOrderRepository.FindPaidWorkHistoryByProviderIDs(
		t.Context(),
		[]int{firstFixture.providerID, secondFixture.providerID, thirdFixture.providerID},
	)

	require.NoError(t, err)
	require.Len(t, historyByProviderID[firstFixture.providerID], 1)
	require.Len(t, historyByProviderID[secondFixture.providerID], 1)
	require.Empty(t, historyByProviderID[thirdFixture.providerID])
	assert.Equal(t, firstOrder.ID(), historyByProviderID[firstFixture.providerID][0].ID)
	assert.Equal(t, secondOrder.ID(), historyByProviderID[secondFixture.providerID][0].ID)
}

func TestWorkOrderRepositoryRejectsPaidHistoryWithoutCompletionReport(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "history-inconsistent")
	order := saveScheduledWorkOrderAt(
		t,
		testContext,
		fixture.conversation,
		fixture.consumerID,
		fixture.providerID,
		time.Now().UTC().Truncate(time.Microsecond).Add(48*time.Hour),
	)
	_, err := testContext.database.Exec(
		`UPDATE work_orders
		SET status = $1, paid_on = $2
		WHERE id = $3`,
		workorder.StatusPaid,
		time.Now().UTC().Truncate(time.Microsecond),
		order.ID(),
	)
	require.NoError(t, err)

	_, err = testContext.workOrderRepository.FindPaidWorkHistoryByProviderID(t.Context(), fixture.providerID)

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)
}

func TestWorkOrderRepositoryFindsOnlyOrdersScheduledInsideWindow(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		database:       testContext.database,
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

func TestWorkOrderRepositoryFindsFutureOrdersWithCalendarData(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "calendar-future")
	from := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	saveScheduledWorkOrderAt(
		t,
		testContext,
		fixture.conversation,
		fixture.consumerID,
		fixture.providerID,
		from.Add(-time.Hour),
	)
	futureOrder := saveScheduledWorkOrderAt(
		t,
		testContext,
		fixture.conversation,
		fixture.consumerID,
		fixture.providerID,
		from.Add(time.Hour),
	)

	orders, err := testContext.workOrderRepository.FindScheduledAfter(t.Context(), from)

	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, futureOrder.ID(), orders[0].ID())
	assert.Equal(t, "Ana", orders[0].Consumer().Name())
	assert.Equal(t, "Perez", orders[0].Consumer().Surname())
	assert.Equal(t, "Juan", orders[0].Provider().Name())
	assert.Equal(t, "Gomez", orders[0].Provider().Surname())
	assert.Equal(t, "Urgent work order repository test.", orders[0].Description())
	assert.Equal(t, 60, orders[0].EstimatedDurationMinutes())
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

func savePaidWorkOrderWithReview(
	t *testing.T,
	testContext serviceProposalRepositoryTestContext,
) (*workorder.WorkOrder, *workorder.Review) {
	t.Helper()
	consumerAuthID := "auth0|review-repository-consumer"
	providerAuthID := "auth0|review-repository-provider"
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		database:       testContext.database,
		userRepository: testContext.userRepository,
	}, consumerAuthID, "review.repository.consumer@example.com", "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, providerAuthID, "review.repository.provider@example.com", "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(t.Context(), activeConversation)
	require.NoError(t, err)

	scheduledOn := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	order := saveScheduledWorkOrderAt(t, testContext, activeConversation, consumerID, providerID, scheduledOn)
	completionImageID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	saveReviewCompletionImage(t, testContext, providerAuthID, completionImageID)
	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{completionImageID}, scheduledOn)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(providerID, report))
	require.NoError(t, order.RegisterApprovedBalancePayment(scheduledOn.Add(time.Hour)))

	reviewer := &consumer.Consumer{
		BaseUser: user.RehydrateBaseUser(
			consumerID,
			consumerAuthID,
			"review.repository.consumer@example.com",
			"Ana",
			"Perez",
			consumer.Role,
			nil,
		),
	}
	review, err := workorder.NewReview(5, "  Trabajo prolijo y claro.  ")
	require.NoError(t, err)
	require.NoError(t, order.AddReview(reviewer, review))

	return order, review
}

func saveReviewCompletionImage(
	t *testing.T,
	testContext serviceProposalRepositoryTestContext,
	providerAuthID string,
	fileID string,
) {
	t.Helper()

	_, err := testContext.database.Exec(
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, 'private', 'trabajo.jpg', 'image/jpeg', 1024, 'confirmed', 'private', 'work_order_completion_image', $3, NOW(), NOW())`,
		fileID,
		"files/2026/08/work_order_completion_image/"+fileID+"/trabajo.jpg",
		providerAuthID,
	)
	require.NoError(t, err)
}

type providerWorkOrderTestFixture struct {
	consumerID     int
	consumerAuthID string
	providerID     int
	providerAuthID string
	conversation   conversation.Conversation
}

func newProviderWorkOrderTestFixture(
	t *testing.T,
	testContext serviceProposalRepositoryTestContext,
	suffix string,
) providerWorkOrderTestFixture {
	t.Helper()
	consumerAuthID := fmt.Sprintf("auth0|provider-work-order-consumer-%s", suffix)
	providerAuthID := fmt.Sprintf("auth0|provider-work-order-provider-%s", suffix)
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		database:       testContext.database,
		userRepository: testContext.userRepository,
	}, consumerAuthID, fmt.Sprintf("provider.work.order.consumer.%s@example.com", suffix), "Ana", "Perez")
	providerID := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database:           testContext.database,
		userRepository:     testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, providerAuthID, fmt.Sprintf("provider.work.order.provider.%s@example.com", suffix), "Juan", "Gomez", "Plomeria")
	activeConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	require.NoError(t, err)
	require.NoError(t, activeConversation.Activate())
	activeConversation, err = testContext.conversationRepository.SaveConversation(t.Context(), activeConversation)
	require.NoError(t, err)

	return providerWorkOrderTestFixture{
		consumerID:     consumerID,
		consumerAuthID: consumerAuthID,
		providerID:     providerID,
		providerAuthID: providerAuthID,
		conversation:   activeConversation,
	}
}

func savePaidWorkOrderWithoutReviewForFixture(
	t *testing.T,
	testContext serviceProposalRepositoryTestContext,
	fixture providerWorkOrderTestFixture,
	scheduledOn time.Time,
	completionImageID string,
) *workorder.WorkOrder {
	t.Helper()
	order := saveScheduledWorkOrderAt(
		t,
		testContext,
		fixture.conversation,
		fixture.consumerID,
		fixture.providerID,
		scheduledOn,
	)
	saveReviewCompletionImage(t, testContext, fixture.providerAuthID, completionImageID)
	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{completionImageID}, scheduledOn)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(fixture.providerID, report))
	require.NoError(t, order.RegisterApprovedBalancePayment(scheduledOn.Add(time.Hour)))
	savedOrder, err := testContext.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	return savedOrder
}

func savePaidWorkOrderWithReviewForFixture(
	t *testing.T,
	testContext serviceProposalRepositoryTestContext,
	fixture providerWorkOrderTestFixture,
	scheduledOn time.Time,
	completionImageID string,
	rating int,
	description string,
) *workorder.WorkOrder {
	t.Helper()
	order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, scheduledOn, completionImageID)
	review, err := workorder.NewReview(rating, description)
	require.NoError(t, err)
	reviewer := &consumer.Consumer{
		BaseUser: user.RehydrateBaseUser(
			fixture.consumerID,
			fixture.consumerAuthID,
			"",
			"Ana",
			"Perez",
			consumer.Role,
			nil,
		),
	}
	require.NoError(t, order.AddReview(reviewer, review))
	savedOrder, err := testContext.workOrderRepository.Save(t.Context(), order)
	require.NoError(t, err)
	return savedOrder
}
