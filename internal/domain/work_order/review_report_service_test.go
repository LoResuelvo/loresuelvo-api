package workorder_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReportReviewRejectsIneligibleActorBeforeLoadingOrder(t *testing.T) {
	for _, tc := range []struct {
		name, role         string
		id                 int
		err, errorExpected error
	}{
		{"consumer", "consumer", 0, nil, workorder.ErrReviewReportForbidden}, {"missing", "", 0, user.ErrNotFound, workorder.ErrReviewReportForbidden}, {"admin", "admin", 0, nil, workorder.ErrReviewReportForbidden}, {"missing profile", "provider", 0, nil, workorder.ErrReviewReportForbidden}, {"database failure", "", 0, errors.New("db"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actors := new(reviewReportActorFinderMock)
			actors.On("FindByAuthID", mock.Anything, "auth").Return(tc.id, tc.role, tc.err).Once()
			service := workorder.NewReportReviewService(actors, nil, nil, nil)
			_, err := service.Report(t.Context(), "auth", 1, "personal_data", "")
			if tc.err != nil && tc.err != user.ErrNotFound {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.ErrorIs(t, err, tc.errorExpected)
			}
			actors.AssertExpectations(t)
		})
	}
}
func TestReportReviewService(t *testing.T) {
	for _, scenario := range []string{"success", "save failure", "existing handled report", "missing review", "wrong provider", "order database failure", "report lookup failure", "missing order"} {
		t.Run(scenario, func(t *testing.T) {
			order, reviewer := paidWorkOrderForReview(t)
			if scenario != "missing review" {
				require.NoError(t, order.AddReview(reviewer, reviewForService(t)))
			}
			actorID := order.ServiceProposal().ProviderID()
			if scenario == "wrong provider" {
				actorID++
			}
			actors := new(reviewReportActorFinderMock)
			actors.On("FindByAuthID", mock.Anything, "auth").Return(actorID, "provider", nil).Once()
			orders := new(readerMock)
			reports := new(reviewReportRepositoryMock)
			clock := new(clockMock)
			orderErr := error(nil)
			expectedErr := error(nil)
			foundOrder := order
			switch scenario {
			case "wrong provider":
				expectedErr = workorder.ErrReviewReportForbidden
			case "order database failure":
				orderErr = errors.New("order db")
				expectedErr = orderErr
			case "missing order":
				foundOrder = nil
				expectedErr = workorder.ErrReviewNotAvailable
			}
			orders.On("FindByID", mock.Anything, order.ID()).Return(foundOrder, orderErr).Once()
			if scenario != "wrong provider" && scenario != "order database failure" && scenario != "missing order" {
				lookupErr := workorder.ErrReviewReportNotFound
				var prior *workorder.ReviewReport
				if scenario == "existing handled report" {
					var err error
					prior, err = workorder.RestoreReviewReport(9, order.ID(), actorID, "personal_data", "", "dismissed", time.Now())
					require.NoError(t, err)
					lookupErr = nil
					expectedErr = workorder.ErrReviewReportAlreadyExists
				}
				if scenario == "report lookup failure" {
					lookupErr = errors.New("report db")
					expectedErr = lookupErr
				}
				reports.On("FindByWorkOrderID", mock.Anything, order.ID()).Return(prior, lookupErr).Once()
				if scenario != "existing handled report" && scenario != "report lookup failure" {
					clock.On("Now").Return(time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)).Once()
					if scenario == "missing review" {
						expectedErr = workorder.ErrReviewNotAvailable
					} else {
						saveErr := error(nil)
						if scenario == "save failure" {
							saveErr = errors.New("save db")
							expectedErr = saveErr
						}
						reports.On("Save", mock.Anything, mock.MatchedBy(func(r *workorder.ReviewReport) bool {
							return r.ReporterID() == actorID && r.WorkOrderID() == order.ID() && r.Explanation() == "private text" && r.Status() == "pending"
						})).Return(saveErr).Once()
					}
				}
			}
			service := workorder.NewReportReviewService(actors, orders, reports, clock)
			result, err := service.Report(t.Context(), "auth", order.ID(), "personal_data", "  private text  ")
			if expectedErr != nil {
				require.ErrorIs(t, err, expectedErr)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
			actors.AssertExpectations(t)
			orders.AssertExpectations(t)
			reports.AssertExpectations(t)
			clock.AssertExpectations(t)
		})
	}
}

func TestReportReviewRejectsInvalidExplanationWithoutPersistence(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	require.NoError(t, order.AddReview(reviewer, reviewForService(t)))
	actors := new(reviewReportActorFinderMock)
	actors.On("FindByAuthID", mock.Anything, "auth").Return(order.ServiceProposal().ProviderID(), "provider", nil).Once()
	orders := new(readerMock)
	orders.On("FindByID", mock.Anything, order.ID()).Return(order, nil).Once()
	reports := new(reviewReportRepositoryMock)
	reports.On("FindByWorkOrderID", mock.Anything, order.ID()).Return(nil, workorder.ErrReviewReportNotFound).Once()
	clock := new(clockMock)
	clock.On("Now").Return(time.Now()).Once()
	service := workorder.NewReportReviewService(actors, orders, reports, clock)
	_, err := service.Report(t.Context(), "auth", order.ID(), "personal_data", "bad\x00text")
	require.ErrorIs(t, err, workorder.ErrInvalidReviewReport)
	reports.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	actors.AssertExpectations(t)
	orders.AssertExpectations(t)
	reports.AssertExpectations(t)
	clock.AssertExpectations(t)
}
