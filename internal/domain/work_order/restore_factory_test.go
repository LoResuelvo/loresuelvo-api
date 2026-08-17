package workorder_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreFactoryRestoresScheduledWorkOrder(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	proposal := &serviceproposal.ServiceProposal{ID: 42}

	order, err := (workorder.RestoreFactory{}).Restore(workorder.RestoreInput{
		ID:              84,
		ServiceProposal: proposal,
		Status:          workorder.StatusScheduled,
		AcceptedOn:      acceptedOn,
	})

	require.NoError(t, err)
	assert.Equal(t, 84, order.ID())
	assert.Equal(t, proposal, order.ServiceProposal())
	assert.Equal(t, workorder.StatusScheduled, order.Status())
	assert.Equal(t, acceptedOn, order.AcceptedOn())
}

func TestRestoreFactoryRestoresPaidWorkOrder(t *testing.T) {
	reportedOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order, err := (workorder.RestoreFactory{}).Restore(
		workorder.RestoreInput{
			ID:              84,
			ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
			Status:          workorder.StatusPaid,
			AcceptedOn:      reportedOn.Add(-time.Hour),
			CompletionReport: &workorder.CompletionReportRestoreInput{
				ID:           9,
				Description:  "Trabajo terminado",
				ImageFileIDs: []string{"file-1"},
				ReportedOn:   reportedOn,
			},
			PaidOn: reportedOn.Add(time.Hour),
		},
	)

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, order.Status())
	assert.Equal(t, 9, order.CompletionReport().ID())
	assert.Equal(t, reportedOn, order.CompletionReport().ReportedOn())
	assert.Equal(t, reportedOn.Add(time.Hour), order.PaidOn())
}

func TestRestoreFactoryRestoresPaidWorkOrderWithReview(t *testing.T) {
	reportedOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	order, err := (workorder.RestoreFactory{}).Restore(
		workorder.RestoreInput{
			ID:              84,
			ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
			Status:          workorder.StatusPaid,
			AcceptedOn:      reportedOn.Add(-time.Hour),
			CompletionReport: &workorder.CompletionReportRestoreInput{
				ID:           9,
				Description:  "Trabajo terminado",
				ImageFileIDs: []string{"file-1"},
				ReportedOn:   reportedOn,
			},
			PaidOn: reportedOn.Add(time.Hour),
			Review: &workorder.ReviewRestoreInput{
				Rating:      4,
				Description: "  Trabajo correcto  ",
			},
		},
	)

	require.NoError(t, err)
	require.NotNil(t, order.Review())
	assert.Equal(t, 4, order.Review().Rating())
	assert.Equal(t, "Trabajo correcto", order.Review().Description())
}

func TestRestoreFactoryRejectsInvalidIdentity(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(workorder.RestoreInput{
		ID:              0,
		ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
		Status:          workorder.StatusScheduled,
		AcceptedOn:      time.Now().UTC(),
	})

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderIdentity)
}

func TestRestoreFactoryRejectsMissingProposal(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(workorder.RestoreInput{
		ID:         84,
		Status:     workorder.StatusScheduled,
		AcceptedOn: time.Now().UTC(),
	})

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderIdentity)
}

func TestRestoreFactoryRejectsUnknownState(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(workorder.RestoreInput{
		ID:              84,
		ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
		Status:          workorder.Status("unknown"),
		AcceptedOn:      time.Now().UTC(),
	})

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)
}

func TestRestoreFactoryRejectsInconsistentCompletionEvidence(t *testing.T) {
	base := workorder.RestoreInput{
		ID:              84,
		ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
		AcceptedOn:      time.Now().UTC(),
	}
	report := &workorder.CompletionReportRestoreInput{
		ID:           9,
		Description:  "Trabajo terminado",
		ImageFileIDs: []string{"file-1"},
		ReportedOn:   time.Now().UTC(),
	}

	base.Status = workorder.StatusScheduled
	base.CompletionReport = report
	_, err := (workorder.RestoreFactory{}).Restore(base)
	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)

	base.Status = workorder.StatusAwaitingPayment
	base.CompletionReport = nil
	_, err = (workorder.RestoreFactory{}).Restore(base)
	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)

	base.Status = workorder.StatusPaid
	base.CompletionReport = report
	base.PaidOn = time.Time{}
	_, err = (workorder.RestoreFactory{}).Restore(base)
	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)
}

func TestRestoreFactoryRejectsReviewForUnpaidWorkOrder(t *testing.T) {
	base := workorder.RestoreInput{
		ID:              84,
		ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
		AcceptedOn:      time.Now().UTC(),
		Review: &workorder.ReviewRestoreInput{
			Rating:      5,
			Description: "Trabajo correcto",
		},
	}

	for _, status := range []workorder.Status{workorder.StatusScheduled, workorder.StatusAwaitingPayment} {
		t.Run(string(status), func(t *testing.T) {
			input := base
			input.Status = status
			if status == workorder.StatusAwaitingPayment {
				input.CompletionReport = &workorder.CompletionReportRestoreInput{
					ID:           9,
					Description:  "Trabajo terminado",
					ImageFileIDs: []string{"file-1"},
					ReportedOn:   time.Now().UTC(),
				}
			}

			_, err := (workorder.RestoreFactory{}).Restore(input)

			assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)
		})
	}
}

func TestRestoreFactoryRejectsInvalidReview(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(workorder.RestoreInput{
		ID:              84,
		ServiceProposal: &serviceproposal.ServiceProposal{ID: 42},
		Status:          workorder.StatusPaid,
		AcceptedOn:      time.Now().UTC(),
		CompletionReport: &workorder.CompletionReportRestoreInput{
			ID:           9,
			Description:  "Trabajo terminado",
			ImageFileIDs: []string{"file-1"},
			ReportedOn:   time.Now().UTC(),
		},
		PaidOn: time.Now().UTC(),
		Review: &workorder.ReviewRestoreInput{
			Rating: 0,
		},
	})

	assert.ErrorIs(t, err, workorder.ErrReviewRatingOutOfRange)
}
