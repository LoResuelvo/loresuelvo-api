package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSemanticTemplateBindsExactOutputAndCriterion(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	require.Len(t, document.Reviews, 1)
	require.Equal(t, digest([]byte(attempt.RawOutput)), document.Reviews[0].OutputSHA256)
	require.Equal(t, "unassessed", document.Reviews[0].Result)
	require.Equal(t, record.RunID, document.RunID)
	require.NotEmpty(t, document.AttemptsSHA256)
	require.NotEmpty(t, document.Reviews[0].CriterionSHA256)
}
func TestSemanticHumanReviewDoesNotApproveRelease(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	dataset.Suites["critical_all"] = []string{"PD-critical-missing"}
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	now := time.Now().UTC()
	document.Reviews[0].Result = "pass"
	document.Reviews[0].Evidence = "Reasons match supplied evidence."
	document.Reviews[0].Reviewer = "team-member"
	document.Reviews[0].ReviewerKind = "human"
	document.Reviews[0].ReviewedOn = &now
	report, err := ApplySemanticReviews(dataset, directory, document)
	require.NoError(t, err)
	require.Equal(t, "human_reviewed", report.Report.SemanticStatus)
	require.Zero(t, report.PendingHumanChecks)
	require.False(t, report.ReleaseApproved)
	require.False(t, report.Report.ReleaseApproved)
	require.Equal(t, []string{"PD-critical-missing"}, report.MissingCriticalCases)
	_, unchanged, err := Replay(dataset, directory)
	require.NoError(t, err)
	require.Equal(t, "unassessed", unchanged.Attempts[0].Evaluation.SemanticChecks[0].Result)
}
func TestSemanticAgentObservationRemainsVisibleWithoutCertification(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	now := time.Now().UTC()
	document.Reviews[0].Result = "pass"
	document.Reviews[0].Evidence = "Observed grounded reasons."
	document.Reviews[0].Reviewer = "codex"
	document.Reviews[0].ReviewerKind = "agent"
	document.Reviews[0].ReviewedOn = &now
	report, err := ApplySemanticReviews(dataset, directory, document)
	require.NoError(t, err)
	require.Equal(t, "pass", report.Report.Attempts[0].Evaluation.SemanticChecks[0].Result)
	require.Equal(t, 1, report.PendingHumanChecks)
	require.Equal(t, "needs_human_review", report.Report.SemanticStatus)
	require.Equal(t, "agent", report.Reviews[0].ReviewerKind)
	require.False(t, report.ReleaseApproved)
}
func TestSemanticReviewRejectsInvalidBindingsAndProvenance(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	directory := persistReplayEvidence(t, record, attempt)
	cases := []struct {
		name   string
		mutate func(*SemanticReviewDocument)
	}{
		{"run", func(d *SemanticReviewDocument) { d.RunID = "other" }},
		{"journal", func(d *SemanticReviewDocument) { d.AttemptsSHA256 = "other" }},
		{"output", func(d *SemanticReviewDocument) { d.Reviews[0].OutputSHA256 = "other" }},
		{"criterion", func(d *SemanticReviewDocument) { d.Reviews[0].Criterion = "changed" }},
		{"severity", func(d *SemanticReviewDocument) { d.Reviews[0].Severity = "critical" }},
		{"unknown", func(d *SemanticReviewDocument) { d.Reviews[0].CriterionID = "other" }},
		{"duplicate", func(d *SemanticReviewDocument) { d.Reviews = append(d.Reviews, d.Reviews[0]) }},
		{"result", func(d *SemanticReviewDocument) { d.Reviews[0].Result = "approved" }},
		{"evidence", func(d *SemanticReviewDocument) { d.Reviews[0].Evidence = "" }},
		{"reviewer", func(d *SemanticReviewDocument) { d.Reviews[0].Reviewer = "" }},
		{"kind", func(d *SemanticReviewDocument) { d.Reviews[0].ReviewerKind = "expert-guessed" }},
		{"time", func(d *SemanticReviewDocument) { d.Reviews[0].ReviewedOn = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, err := NewSemanticReviewTemplate(dataset, directory)
			require.NoError(t, err)
			now := time.Now().UTC()
			review := &document.Reviews[0]
			review.Result = "pass"
			review.Evidence = "evidence"
			review.Reviewer = "reviewer"
			review.ReviewerKind = "human"
			review.ReviewedOn = &now
			tc.mutate(&document)
			_, err = ApplySemanticReviews(dataset, directory, document)
			require.ErrorIs(t, err, ErrInvalidSemanticReview)
		})
	}
}
func TestSemanticMissingOutputCannotReceivePass(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	attempt.Status = "execution_error"
	attempt.Error = "timeout"
	attempt.RawOutput = ""
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	document.Reviews[0].Result = "pass"
	_, err = ApplySemanticReviews(dataset, directory, document)
	require.ErrorContains(t, err, "failed execution")
}
func TestSemanticFailureOverridesDeterministicPass(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	now := time.Now().UTC()
	review := &document.Reviews[0]
	review.Result = "fail"
	review.Evidence = "Unsupported fact in reason."
	review.Reviewer = "reviewer"
	review.ReviewerKind = "agent"
	review.ReviewedOn = &now
	report, err := ApplySemanticReviews(dataset, directory, document)
	require.NoError(t, err)
	require.Equal(t, "failed", report.Report.Attempts[0].Evaluation.OverallStatus)
	require.Equal(t, "passed", report.Report.Attempts[0].Evaluation.DeterministicStatus)
	require.Equal(t, "failed", report.Report.SemanticStatus)
}
func TestSemanticReviewFileRoundTripDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviews.json")
	document := SemanticReviewDocument{FormatVersion: semanticReviewVersion, RunID: "run"}
	require.NoError(t, WriteSemanticReviews(path, document))
	actual, err := ReadSemanticReviews(path)
	require.NoError(t, err)
	require.Equal(t, document, actual)
	require.ErrorIs(t, WriteSemanticReviews(path, document), os.ErrExist)
}
func TestSemanticReadRejectsUnknownAndTrailingData(t *testing.T) {
	for _, raw := range []string{`{"unknown":true}`, `{} {}`} {
		path := filepath.Join(t.TempDir(), "reviews.json")
		require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
		_, err := ReadSemanticReviews(path)
		require.Error(t, err)
	}
}
func TestSemanticPartialImportKeepsCriticalPending(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	dataset.RK[0].Expected = json.RawMessage(`{"eligible_references":["a","b","c"],"relevance":{"a":3,"b":2,"c":0},"semantic_assertions":[{"id":"critical-check","severity":"critical","criterion":"Review safety."}]}`)
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	document.Reviews = nil
	report, err := ApplySemanticReviews(dataset, directory, document)
	require.NoError(t, err)
	require.Equal(t, 1, report.CriticalPendingChecks)
	require.Equal(t, 1, report.ResultCounts["unassessed"])
}

func TestSemanticNotApplicableRequiresExplainedProvenance(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	now := time.Now().UTC()
	review := &document.Reviews[0]
	review.Result = "not_applicable"
	review.Evidence = "Criterion does not apply to this response because it makes no factual assertion."
	review.Reviewer = "reviewer"
	review.ReviewerKind = "human"
	review.ReviewedOn = &now
	report, err := ApplySemanticReviews(dataset, directory, document)
	require.NoError(t, err)
	require.Equal(t, 1, report.ResultCounts["not_applicable"])
	require.False(t, report.ReleaseApproved)
}

func TestSemanticAbsentExecutedOutputRemainsUnassessed(t *testing.T) {
	for _, raw := range []string{"", " \n\t ", "null", " \nnull\t "} {
		t.Run(raw, func(t *testing.T) {
			dataset, record, attempt := replayEvidence(t)
			attempt.RawOutput = raw
			directory := persistReplayEvidence(t, record, attempt)
			document, err := NewSemanticReviewTemplate(dataset, directory)
			require.NoError(t, err)
			report, err := ApplySemanticReviews(dataset, directory, document)
			require.NoError(t, err)
			require.Equal(t, 1, report.ResultCounts["unassessed"])
			now := time.Now().UTC()
			review := &document.Reviews[0]
			review.Evidence = "No response to evaluate."
			review.Reviewer = "reviewer"
			review.ReviewerKind = "human"
			review.ReviewedOn = &now
			for _, status := range []string{"pass", "fail", "not_applicable"} {
				review.Result = status
				_, err = ApplySemanticReviews(dataset, directory, document)
				require.ErrorIs(t, err, ErrInvalidSemanticReview)
			}
		})
	}
}
func TestSemanticMeaningfulSchemaInvalidOutputRemainsReviewable(t *testing.T) {
	dataset, record, attempt := replayEvidence(t)
	attempt.RawOutput = "Recommendation claims unsupported experience."
	directory := persistReplayEvidence(t, record, attempt)
	document, err := NewSemanticReviewTemplate(dataset, directory)
	require.NoError(t, err)
	now := time.Now().UTC()
	review := &document.Reviews[0]
	review.Result = "fail"
	review.Evidence = "Unsupported experience assertion in response."
	review.Reviewer = "reviewer"
	review.ReviewerKind = "human"
	review.ReviewedOn = &now
	report, err := ApplySemanticReviews(dataset, directory, document)
	require.NoError(t, err)
	require.Equal(t, 1, report.ResultCounts["fail"])
	require.Equal(t, "failed", report.Report.Attempts[0].Evaluation.DeterministicStatus)
}
