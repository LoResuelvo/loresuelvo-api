package evals

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const semanticReviewVersion = "1"

var ErrInvalidSemanticReview = errors.New("invalid semantic review")

// SemanticReview binds a judgment to one exact response and criterion. Reviewer
// identity is declared provenance, not authenticated or specialist certification.
type SemanticReview struct {
	CaseID          string     `json:"case_id"`
	Trial           int        `json:"trial"`
	Retry           int        `json:"retry"`
	OutputSHA256    string     `json:"output_sha256"`
	CriterionID     string     `json:"criterion_id"`
	CriterionSHA256 string     `json:"criterion_sha256"`
	Criterion       string     `json:"criterion"`
	Severity        string     `json:"severity"`
	Result          string     `json:"result"`
	Evidence        string     `json:"evidence"`
	Reviewer        string     `json:"reviewer"`
	ReviewerKind    string     `json:"reviewer_kind"`
	ReviewedOn      *time.Time `json:"reviewed_on"`
}
type SemanticReviewDocument struct {
	FormatVersion  string           `json:"format_version"`
	RunID          string           `json:"run_id"`
	DatasetSHA256  string           `json:"dataset_sha256"`
	AttemptsSHA256 string           `json:"attempts_sha256"`
	Reviews        []SemanticReview `json:"reviews"`
}
type SemanticReviewReport struct {
	Report                Report           `json:"report"`
	Reviews               []SemanticReview `json:"reviews"`
	ResultCounts          map[string]int   `json:"result_counts"`
	PendingHumanChecks    int              `json:"pending_human_checks"`
	CriticalPendingChecks int              `json:"critical_pending_checks"`
	MissingCriticalCases  []string         `json:"missing_critical_cases"`
	ReleaseApproved       bool             `json:"release_approved"`
	Warning               string           `json:"warning"`
}

func NewSemanticReviewTemplate(dataset *Dataset, runDirectory string) (SemanticReviewDocument, error) {
	record, report, err := Replay(dataset, runDirectory)
	if err != nil {
		return SemanticReviewDocument{}, err
	}
	observed, attempts, err := ReadRun(runDirectory)
	if err != nil {
		return SemanticReviewDocument{}, err
	}
	if observed.AttemptsSHA256 != record.AttemptsSHA256 || len(attempts) != len(report.Attempts) {
		return SemanticReviewDocument{}, fmt.Errorf("%w: run changed during review", ErrInvalidSemanticReview)
	}
	document := SemanticReviewDocument{FormatVersion: semanticReviewVersion, RunID: record.RunID, DatasetSHA256: record.Plan.DatasetSHA256, AttemptsSHA256: record.AttemptsSHA256, Reviews: []SemanticReview{}}
	for i, attempt := range report.Attempts {
		for _, check := range attempt.Evaluation.SemanticChecks {
			document.Reviews = append(document.Reviews, SemanticReview{CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, OutputSHA256: digest([]byte(attempts[i].RawOutput)), CriterionID: check.ID, CriterionSHA256: semanticCriterionHash(check), Criterion: check.Criterion, Severity: check.Severity, Result: "unassessed"})
		}
	}
	return document, nil
}

// ApplySemanticReviews is offline and never approves a release, even when every
// imported judgment is positive. Agent-only judgments remain pending human review.
func ApplySemanticReviews(dataset *Dataset, runDirectory string, document SemanticReviewDocument) (SemanticReviewReport, error) {
	template, err := NewSemanticReviewTemplate(dataset, runDirectory)
	if err != nil {
		return SemanticReviewReport{}, err
	}
	if document.FormatVersion != template.FormatVersion || document.RunID != template.RunID || document.DatasetSHA256 != template.DatasetSHA256 || document.AttemptsSHA256 != template.AttemptsSHA256 {
		return SemanticReviewReport{}, fmt.Errorf("%w: document does not match verified run", ErrInvalidSemanticReview)
	}
	record, report, err := Replay(dataset, runDirectory)
	if err != nil {
		return SemanticReviewReport{}, err
	}
	if record.AttemptsSHA256 != template.AttemptsSHA256 {
		return SemanticReviewReport{}, fmt.Errorf("%w: run changed during review", ErrInvalidSemanticReview)
	}
	expected := make(map[string]SemanticReview, len(template.Reviews))
	for _, review := range template.Reviews {
		key := semanticReviewKey(review)
		if _, exists := expected[key]; exists {
			return SemanticReviewReport{}, fmt.Errorf("%w: ambiguous criterion %s", ErrInvalidSemanticReview, key)
		}
		expected[key] = review
	}
	observedRecord, rawAttempts, err := ReadRun(runDirectory)
	if err != nil {
		return SemanticReviewReport{}, err
	}
	if observedRecord.AttemptsSHA256 != template.AttemptsSHA256 {
		return SemanticReviewReport{}, fmt.Errorf("%w: run changed during review", ErrInvalidSemanticReview)
	}
	statuses := make(map[string]string, len(rawAttempts))
	for _, attempt := range rawAttempts {
		status := attempt.Status
		raw := strings.TrimSpace(attempt.RawOutput)
		if raw == "" || raw == "null" {
			status = "missing_output"
		}
		statuses[semanticAttemptKey(attempt.CaseID, attempt.Trial, attempt.Retry)] = status
	}
	imported := make(map[string]SemanticReview, len(document.Reviews))
	for _, review := range document.Reviews {
		key := semanticReviewKey(review)
		original, exists := expected[key]
		if !exists {
			return SemanticReviewReport{}, fmt.Errorf("%w: unknown criterion %s", ErrInvalidSemanticReview, key)
		}
		if _, duplicate := imported[key]; duplicate {
			return SemanticReviewReport{}, fmt.Errorf("%w: duplicate criterion %s", ErrInvalidSemanticReview, key)
		}
		if err = validateSemanticReview(original, review, statuses[semanticAttemptKey(review.CaseID, review.Trial, review.Retry)]); err != nil {
			return SemanticReviewReport{}, fmt.Errorf("%s: %w", key, err)
		}
		imported[key] = review
	}
	result := SemanticReviewReport{Report: report, Reviews: append([]SemanticReview(nil), document.Reviews...), ResultCounts: map[string]int{"pass": 0, "fail": 0, "unassessed": 0, "not_applicable": 0}, MissingCriticalCases: []string{}, Warning: "Reviewer provenance is self-declared. Agent judgments do not certify safety. This report never approves release; coverage and independent human release assessment remain required."}
	observed := map[string]bool{}
	anyFail := false
	for i := range result.Report.Attempts {
		attempt := &result.Report.Attempts[i]
		if statuses[semanticAttemptKey(attempt.CaseID, attempt.Trial, attempt.Retry)] == "executed" {
			observed[attempt.CaseID] = true
		}
		humanComplete := len(attempt.Evaluation.SemanticChecks) > 0
		semanticFail := false
		for j := range attempt.Evaluation.SemanticChecks {
			check := &attempt.Evaluation.SemanticChecks[j]
			review, exists := imported[semanticReviewKey(SemanticReview{CaseID: attempt.CaseID, Trial: attempt.Trial, Retry: attempt.Retry, CriterionID: check.ID})]
			if exists {
				check.Result = review.Result
			}
			result.ResultCounts[check.Result]++
			if !exists || review.Result == "unassessed" || review.ReviewerKind != "human" {
				humanComplete = false
				result.PendingHumanChecks++
				if check.Severity == "critical" {
					result.CriticalPendingChecks++
				}
			}
			if check.Result == "fail" {
				semanticFail = true
				anyFail = true
			}
		}
		switch {
		case attempt.Evaluation.DeterministicStatus == "failed" || semanticFail:
			attempt.Evaluation.OverallStatus = "failed"
		case humanComplete:
			attempt.Evaluation.OverallStatus = "human_reviewed"
		default:
			attempt.Evaluation.OverallStatus = "needs_semantic_review"
		}
		attempt.Evaluation.ReleaseApproved = false
	}
	for _, id := range dataset.Suites["critical_all"] {
		if !observed[id] {
			result.MissingCriticalCases = append(result.MissingCriticalCases, id)
		}
	}
	slices.Sort(result.MissingCriticalCases)
	switch {
	case len(template.Reviews) == 0:
		result.Report.SemanticStatus = "unassessed"
	case anyFail:
		result.Report.SemanticStatus = "failed"
	case result.PendingHumanChecks > 0:
		result.Report.SemanticStatus = "needs_human_review"
	default:
		result.Report.SemanticStatus = "human_reviewed"
	}
	result.Report.ReleaseApproved = false
	return result, nil
}
func semanticAttemptKey(id string, trial, retry int) string {
	return fmt.Sprintf("%s/%d/%d", id, trial, retry)
}
func semanticReviewKey(review SemanticReview) string {
	return semanticAttemptKey(review.CaseID, review.Trial, review.Retry) + "/" + review.CriterionID
}
func semanticCriterionHash(check SemanticCheck) string {
	// Length-prefix each component to avoid ambiguous concatenations.
	return digest([]byte(fmt.Sprintf("%d:%s%d:%s%d:%s", len(check.ID), check.ID, len(check.Severity), check.Severity, len(check.Criterion), check.Criterion)))
}
func validateSemanticReview(expected, review SemanticReview, status string) error {
	if expected.OutputSHA256 != review.OutputSHA256 || expected.CriterionSHA256 != review.CriterionSHA256 || expected.Criterion != review.Criterion || expected.Severity != review.Severity {
		return fmt.Errorf("%w: response or criterion mismatch", ErrInvalidSemanticReview)
	}
	if !slices.Contains([]string{"pass", "fail", "unassessed", "not_applicable"}, review.Result) {
		return fmt.Errorf("%w: unsupported result", ErrInvalidSemanticReview)
	}
	if review.Result == "unassessed" {
		return nil
	}
	if status != "executed" {
		return fmt.Errorf("%w: missing or failed execution cannot receive assessed semantics", ErrInvalidSemanticReview)
	}
	if strings.TrimSpace(review.Evidence) == "" || strings.TrimSpace(review.Reviewer) == "" || review.ReviewedOn == nil || review.ReviewedOn.IsZero() {
		return fmt.Errorf("%w: evidence, reviewer and review time required", ErrInvalidSemanticReview)
	}
	if review.ReviewerKind != "human" && review.ReviewerKind != "agent" {
		return fmt.Errorf("%w: reviewer kind must be human or agent", ErrInvalidSemanticReview)
	}
	_, offset := review.ReviewedOn.Zone()
	if offset != 0 {
		return fmt.Errorf("%w: review time must use UTC", ErrInvalidSemanticReview)
	}
	return nil
}
