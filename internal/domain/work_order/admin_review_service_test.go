package workorder_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdminReviewGetFailsClosedWhenAccessAuditCannotPersist(t *testing.T) {
	reader := new(adminReviewReaderMock)
	operators := new(reviewOperatorFinderMock)
	events := new(reviewAuditWriterMock)
	clock := new(clockMock)
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	operators.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(7, nil).Once()
	original, err := workorder.NewReview(5, "restricted original")
	require.NoError(t, err)
	reader.On("FindByID", mock.Anything, 1, workorder.ReviewPageInput{Page: 1, Limit: 20}).Return(&workorder.AdminReviewDetail{Review: original}, nil).Once()
	clock.On("Now").Return(now).Once()
	failure := errors.New("private audit failure")
	events.On("Save", mock.Anything, mock.MatchedBy(func(e *audit.Event) bool {
		return e.ResourceType() == "review" && e.ResourceID() == "1" && e.OperatorID() == 7 && e.Action() == audit.ActionAccess && e.Result() == audit.ResultPrepared && e.CorrelationID() == "request" && e.Reason() == nil && e.OccurredOn().Equal(now)
	})).Return(failure).Once()
	service := workorder.NewAdminReviewService(reader, operators, nil, events, clock)
	result, err := service.Get(t.Context(), "admin", 1, workorder.ReviewPageInput{}, "request")
	require.ErrorIs(t, err, failure)
	require.Nil(t, result)
	reader.AssertExpectations(t)
	operators.AssertExpectations(t)
	clock.AssertExpectations(t)
	events.AssertExpectations(t)
}
func TestAdminReviewGetAuditsOnlyAfterSuccessfulRead(t *testing.T) {
	for _, failure := range []error{workorder.ErrReviewNotAvailable, errors.New("db")} {
		reader := new(adminReviewReaderMock)
		operators := new(reviewOperatorFinderMock)
		events := new(reviewAuditWriterMock)
		operators.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(7, nil).Once()
		reader.On("FindByID", mock.Anything, 1, workorder.ReviewPageInput{Page: 1, Limit: 20}).Return(nil, failure).Once()
		result, err := workorder.NewAdminReviewService(reader, operators, nil, events, nil).Get(t.Context(), "admin", 1, workorder.ReviewPageInput{}, "request")
		require.ErrorIs(t, err, failure)
		require.Nil(t, result)
		events.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		reader.AssertExpectations(t)
		operators.AssertExpectations(t)
	}
}
func TestAdminReviewListPreservesReadFailures(t *testing.T) {
	reader := new(adminReviewReaderMock)
	failure := errors.New("db")
	reader.On("FindPage", mock.Anything, workorder.ReviewListInput{Status: "reported", ReviewPageInput: workorder.ReviewPageInput{Page: 1, Limit: 20}}).Return(nil, failure).Once()
	_, err := workorder.NewAdminReviewService(reader, nil, nil, nil, nil).List(t.Context(), workorder.ReviewListInput{})
	require.ErrorIs(t, err, failure)
	reader.AssertExpectations(t)
}
