package workorder_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/require"
)

func TestReviewModerationConsumesVersionAndPreservesOriginal(t *testing.T) {
	review, err := workorder.NewReview(5, "original")
	require.NoError(t, err)
	decision, err := review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "private reason", ExpectedVersion: 1}, nil, 7, time.Now())
	require.NoError(t, err)
	require.False(t, review.Visible())
	require.Equal(t, 2, review.Version())
	require.Equal(t, "original", review.Description())
	require.Equal(t, "hide", decision.Action())
	_, err = review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "private reason", ExpectedVersion: 1}, nil, 7, time.Now())
	require.ErrorIs(t, err, workorder.ErrReviewModerationConflict)
}
func TestReviewModerationRejectsForeignReport(t *testing.T) {
	review, err := workorder.NewReview(5, "original")
	require.NoError(t, err)
	report, err := workorder.RestoreReviewReport(4, 2, 8, "personal_data", "explanation", "pending", time.Now())
	require.NoError(t, err)
	_, err = review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "private reason", ExpectedVersion: 1, ReportID: 4}, report, 7, time.Now())
	require.ErrorIs(t, err, workorder.ErrInvalidReviewModeration)
	require.True(t, review.Visible())
	require.Equal(t, "pending", report.Status())
}

func TestReviewModerationUnhideLinksLatestHideWithoutReopeningReport(t *testing.T) {
	review, err := workorder.NewReview(5, "original")
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	report, err := workorder.RestoreReviewReport(4, 1, 8, "personal_data", "explanation", "pending", now)
	require.NoError(t, err)
	hidden, err := review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "Reason", ExpectedVersion: 1, ReportID: 4}, report, 7, now)
	require.NoError(t, err)
	hidden.SetID(9)
	review.AssociateDecision(hidden)
	require.Equal(t, "upheld", report.Status())
	restored, err := review.Moderate(1, workorder.ModerationInput{Action: "unhide", Reason: "Review complete", ExpectedVersion: 2}, report, 7, now.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, review.Visible())
	require.Equal(t, 3, review.Version())
	require.Equal(t, 9, restored.PreviousHideID())
	require.Equal(t, "upheld", report.Status())
	require.Equal(t, "original", review.Description())
}
func TestReviewModerationOnlyAttendsExplicitPendingReportWhileHidden(t *testing.T) {
	review, err := workorder.NewReview(5, "original")
	require.NoError(t, err)
	now := time.Now()
	report, err := workorder.RestoreReviewReport(4, 1, 8, "personal_data", "explanation", "pending", now)
	require.NoError(t, err)
	hidden, err := review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "Reason", ExpectedVersion: 1}, report, 7, now)
	require.NoError(t, err)
	hidden.SetID(9)
	review.AssociateDecision(hidden)
	require.Equal(t, "pending", report.Status())
	_, err = review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "Reason", ExpectedVersion: 2}, report, 7, now)
	require.ErrorIs(t, err, workorder.ErrReviewModerationConflict)
	require.Equal(t, 2, review.Version())
	decision, err := review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "Report confirmed", ExpectedVersion: 2, ReportID: 4}, report, 7, now)
	require.NoError(t, err)
	require.False(t, review.Visible())
	require.Equal(t, 3, review.Version())
	require.Equal(t, "upheld", report.Status())
	require.Equal(t, 4, decision.ReportID())
}
func TestReviewModerationDismissDoesNotRestoreHiddenVisibility(t *testing.T) {
	review, err := workorder.NewReview(5, "original")
	require.NoError(t, err)
	now := time.Now()
	report, err := workorder.RestoreReviewReport(4, 1, 8, "personal_data", "explanation", "pending", now)
	require.NoError(t, err)
	hidden, err := review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "Reason", ExpectedVersion: 1}, report, 7, now)
	require.NoError(t, err)
	hidden.SetID(9)
	review.AssociateDecision(hidden)
	_, err = review.Moderate(1, workorder.ModerationInput{Action: "dismiss_reports", Reason: "Not upheld", ExpectedVersion: 2, ReportID: 4}, report, 7, now)
	require.NoError(t, err)
	require.False(t, review.Visible())
	require.Equal(t, "dismissed", report.Status())
	require.Equal(t, 9, review.HidingDecisionID())
}
func TestModerationInputNormalizesBoundedPrintableReason(t *testing.T) {
	for _, reason := range []string{"", "  ", strings.Repeat("ñ", 251), "a\x00b", "line\nbreak", string([]byte{255})} {
		_, err := (workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: reason, ExpectedVersion: 1}).Normalize()
		require.ErrorIs(t, err, workorder.ErrInvalidReviewModeration)
	}
	input, err := (workorder.ModerationInput{Action: "hide", Category: "personal_data", Reason: "  " + strings.Repeat("ñ", 250) + "  ", ExpectedVersion: 1}).Normalize()
	require.NoError(t, err)
	require.Len(t, input.Reason, 500)
}
