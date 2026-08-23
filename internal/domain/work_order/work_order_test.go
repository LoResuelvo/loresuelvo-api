package workorder_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduledWorkOrderRejectsApprovedBalancePaymentBeforeCompletion(t *testing.T) {
	order := workOrderFixture(84, 10, 20, time.Now().UTC())

	err := order.RegisterApprovedBalancePayment(time.Now().UTC())

	assert.ErrorIs(t, err, workorder.ErrWorkOrderNotEligibleForFullPayment)
	assert.Equal(t, workorder.StatusScheduled, order.Status())
}

func TestNewWorkOrderStartsScheduled(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)

	proposal := serviceproposal.ServiceProposal{
		ID:                       1,
		EstimatedDurationMinutes: 90,
	}

	order, err := workorder.New(&proposal, acceptedOn)

	assert.NoError(t, err)
	assert.Equal(t, &proposal, order.ServiceProposal())
	assert.Equal(t, 90, order.EstimatedDurationMinutes())
	assert.Equal(t, workorder.StatusScheduled, order.Status())
	assert.Equal(t, acceptedOn, order.AcceptedOn())
}

func TestWorkOrderExposesTheProposalParticipants(t *testing.T) {
	order := workOrderFixture(84, 10, 20, time.Now().UTC())
	proposal := order.ServiceProposal().(*serviceproposal.ServiceProposal)

	assert.Equal(t, proposal.Consumer, order.Consumer())
	assert.Equal(t, proposal.Provider, order.Provider())
}

func TestWorkOrderReportsCompletionAndAwaitsPayment(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	reportedOn := scheduledOn.Add(time.Minute)
	order := workOrderFixture(84, 10, 20, scheduledOn)
	report, err := workorder.NewCompletionReport(
		"Trabajo finalizado y funcionamiento verificado.",
		[]string{"file-1", "file-2"},
		reportedOn,
	)
	require.NoError(t, err)

	err = order.ReportCompletion(20, report)

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusAwaitingPayment, order.Status())
	assert.Same(t, report, order.CompletionReport())
	assert.Equal(t, "Trabajo finalizado y funcionamiento verificado.", order.CompletionReport().Description())
	assert.Equal(t, []string{"file-1", "file-2"}, order.CompletionReport().ImageFileIDs())
	assert.Equal(t, reportedOn, order.CompletionReport().ReportedOn())
	assert.True(t, order.PaidOn().IsZero())
}

func TestWorkOrderRejectsCompletionByAnotherProvider(t *testing.T) {
	order := workOrderFixture(84, 10, 20, time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC))
	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	err = order.ReportCompletion(21, report)

	assert.ErrorIs(t, err, workorder.ErrOnlyAssignedProviderCanReport)
	assert.Equal(t, workorder.StatusScheduled, order.Status())
}

func TestWorkOrderRejectsCompletionBeforeScheduledTime(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, scheduledOn)
	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, scheduledOn.Add(-time.Second))
	require.NoError(t, err)

	err = order.ReportCompletion(20, report)

	assert.ErrorIs(t, err, workorder.ErrWorkOrderNotReadyForCompletion)
	assert.Equal(t, workorder.StatusScheduled, order.Status())
}

func TestWorkOrderRejectsDuplicateCompletionReport(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, scheduledOn)
	firstReport, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, scheduledOn)
	require.NoError(t, err)
	secondReport, err := workorder.NewCompletionReport("Otro reporte", []string{"file-2"}, scheduledOn.Add(time.Minute))
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(20, firstReport))

	err = order.ReportCompletion(20, secondReport)

	assert.ErrorIs(t, err, workorder.ErrCompletionReportAlreadyExists)
	assert.Same(t, firstReport, order.CompletionReport())
}

func TestCompletionReportValidatesDescriptionImagesAndTimestamp(t *testing.T) {
	reportedOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		description string
		imageIDs    []string
		reportedOn  time.Time
		expected    error
	}{
		{name: "blank description", description: "  ", imageIDs: []string{"file-1"}, reportedOn: reportedOn, expected: workorder.ErrCompletionReportDescriptionRequired},
		{name: "no images", description: "Trabajo terminado", imageIDs: nil, reportedOn: reportedOn, expected: workorder.ErrCompletionReportImageCount},
		{name: "too many images", description: "Trabajo terminado", imageIDs: []string{"file-1", "file-2", "file-3", "file-4"}, reportedOn: reportedOn, expected: workorder.ErrCompletionReportImageCount},
		{name: "empty image id", description: "Trabajo terminado", imageIDs: []string{""}, reportedOn: reportedOn, expected: workorder.ErrCompletionReportImageRequired},
		{name: "duplicate image", description: "Trabajo terminado", imageIDs: []string{"file-1", "file-1"}, reportedOn: reportedOn, expected: workorder.ErrCompletionReportDuplicateImage},
		{name: "missing timestamp", description: "Trabajo terminado", imageIDs: []string{"file-1"}, reportedOn: time.Time{}, expected: workorder.ErrCompletionReportReportedOnRequired},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := workorder.NewCompletionReport(test.description, test.imageIDs, test.reportedOn)
			assert.ErrorIs(t, err, test.expected)
		})
	}
}

func TestWorkOrderAuthorizesCheckoutOnlyWhileAwaitingPayment(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, scheduledOn)
	checkoutOn := scheduledOn.Add(time.Minute)

	err := order.AuthorizeBalanceCheckout(10, checkoutOn)
	assert.ErrorIs(t, err, workorder.ErrWorkOrderNotAwaitingPayment)

	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, checkoutOn)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(20, report))
	assert.ErrorIs(t, order.AuthorizeBalanceCheckout(11, checkoutOn), workorder.ErrOnlyWorkOrderConsumerCanCheckout)
	assert.NoError(t, order.AuthorizeBalanceCheckout(10, checkoutOn))
}

func TestWorkOrderRegistersApprovedBalancePaymentAndKeepsReport(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	paidOn := scheduledOn.Add(2 * time.Minute)
	order := workOrderFixture(84, 10, 20, scheduledOn)
	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, scheduledOn)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(20, report))

	err = order.RegisterApprovedBalancePayment(paidOn)

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, order.Status())
	assert.Same(t, report, order.CompletionReport())
	assert.Equal(t, paidOn, order.PaidOn())
	assert.ErrorIs(t, order.RegisterApprovedBalancePayment(paidOn.Add(time.Minute)), workorder.ErrWorkOrderAlreadyPaid)
}
