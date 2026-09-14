package evals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
)

const semanticCalibrationVersion = "1.0.0"

var ErrInvalidSemanticCalibration = errors.New("invalid semantic calibration")

// SemanticCalibrationSuite contains explicit gold labels. It is not an
// evaluator: labels must be adjudicated before this offline comparison runs.
type SemanticCalibrationSuite struct {
	FormatVersion string                       `json:"format_version"`
	CalibrationID string                       `json:"calibration_id"`
	Examples      []SemanticCalibrationExample `json:"examples"`
}

type SemanticCalibrationExample struct {
	ID              string `json:"id"`
	Output          string `json:"output"`
	OutputSHA256    string `json:"output_sha256"`
	CriterionID     string `json:"criterion_id"`
	CriterionSHA256 string `json:"criterion_sha256"`
	Criterion       string `json:"criterion"`
	Severity        string `json:"severity"`
	ExpectedResult  string `json:"expected_result"`
	ExpectedReason  string `json:"expected_reason"`
	InputContext    string `json:"input_context"`
}

type SemanticCalibrationAssessments struct {
	FormatVersion         string                      `json:"format_version"`
	CalibrationID         string                      `json:"calibration_id"`
	Reviewer              string                      `json:"reviewer"`
	ReviewerKind          string                      `json:"reviewer_kind"`
	ReviewedOn            *time.Time                  `json:"reviewed_on"`
	Reviews               []SemanticCalibrationReview `json:"reviews"`
	CalibrationSHA256     string                      `json:"calibration_sha256"`
	AuthoredReference     bool                        `json:"authored_reference"`
	IndependentAssessment bool                        `json:"independent_assessment"`
	Note                  string                      `json:"note"`
}

type SemanticCalibrationReview struct {
	ExampleID        string `json:"example_id"`
	Result           string `json:"result"`
	EvidenceQuote    string `json:"evidence_quote,omitempty"`
	EvidenceLocation string `json:"evidence_location"`
	Reason           string `json:"reason"`
}

type SemanticCalibrationCounts struct {
	Total        int `json:"total"`
	ExpectedPass int `json:"expected_pass"`
	ExpectedFail int `json:"expected_fail"`
	CorrectPass  int `json:"correct_pass"`
	CorrectFail  int `json:"correct_fail"`
	FalsePass    int `json:"false_pass"`
	FalseFail    int `json:"false_fail"`
	Unassessed   int `json:"unassessed"`
}

type SemanticCalibrationResult struct {
	ExampleID       string `json:"example_id"`
	ExpectedResult  string `json:"expected_result"`
	ObservedResult  string `json:"observed_result"`
	MatchesExpected bool   `json:"matches_expected"`
}

type SemanticCalibrationReport struct {
	FormatVersion     string                      `json:"format_version"`
	CalibrationID     string                      `json:"calibration_id"`
	CalibrationSHA256 string                      `json:"calibration_sha256,omitempty"`
	AssessmentsSHA256 string                      `json:"assessments_sha256,omitempty"`
	Reviewer          string                      `json:"reviewer"`
	ReviewerKind      string                      `json:"reviewer_kind"`
	Counts            SemanticCalibrationCounts   `json:"counts"`
	Agreement         float64                     `json:"agreement"`
	Results           []SemanticCalibrationResult `json:"results"`
	ReleaseApproved   bool                        `json:"release_approved"`
	Warning           string                      `json:"warning"`
}

// CalibrateSemanticReviewer compares supplied judgments with explicit gold
// labels. It validates citations but never derives or changes a semantic label.
func CalibrateSemanticReviewer(suite SemanticCalibrationSuite, assessments SemanticCalibrationAssessments) (SemanticCalibrationReport, error) {
	report := SemanticCalibrationReport{FormatVersion: semanticCalibrationVersion, CalibrationID: suite.CalibrationID, Reviewer: assessments.Reviewer, ReviewerKind: assessments.ReviewerKind, Results: []SemanticCalibrationResult{}, ReleaseApproved: false, Warning: "Offline calibration compares supplied reviews with explicit labels; it does not judge semantics, certify safety, or approve release."}
	if suite.FormatVersion != semanticCalibrationVersion || assessments.FormatVersion != semanticCalibrationVersion || strings.TrimSpace(suite.CalibrationID) == "" || assessments.CalibrationID != suite.CalibrationID {
		return report, fmt.Errorf("%w: identity mismatch", ErrInvalidSemanticCalibration)
	}
	if strings.TrimSpace(assessments.Reviewer) == "" || !slices.Contains([]string{"human", "agent"}, assessments.ReviewerKind) || assessments.ReviewedOn == nil || assessments.ReviewedOn.IsZero() {
		return report, fmt.Errorf("%w: review provenance is incomplete", ErrInvalidSemanticCalibration)
	}
	if assessments.AuthoredReference || !assessments.IndependentAssessment || strings.TrimSpace(assessments.Note) == "" {
		return report, fmt.Errorf("%w: independently produced assessments are required", ErrInvalidSemanticCalibration)
	}
	_, offset := assessments.ReviewedOn.Zone()
	if offset != 0 {
		return report, fmt.Errorf("%w: review time must use UTC", ErrInvalidSemanticCalibration)
	}
	reviews := make(map[string]SemanticCalibrationReview, len(assessments.Reviews))
	for _, review := range assessments.Reviews {
		if strings.TrimSpace(review.ExampleID) == "" || reviews[review.ExampleID].ExampleID != "" {
			return report, fmt.Errorf("%w: duplicate or empty assessment id", ErrInvalidSemanticCalibration)
		}
		reviews[review.ExampleID] = review
	}
	seen := make(map[string]bool, len(suite.Examples))
	for _, example := range suite.Examples {
		if err := validateSemanticCalibrationExample(example); err != nil {
			return report, err
		}
		if seen[example.ID] {
			return report, fmt.Errorf("%w: duplicate example %s", ErrInvalidSemanticCalibration, example.ID)
		}
		seen[example.ID] = true
		review, exists := reviews[example.ID]
		if !exists {
			return report, fmt.Errorf("%w: missing assessment for %s", ErrInvalidSemanticCalibration, example.ID)
		}
		if err := validateCalibrationReview(review, example.Output); err != nil {
			return report, fmt.Errorf("%s: %w", example.ID, err)
		}
		result := SemanticCalibrationResult{ExampleID: example.ID, ExpectedResult: example.ExpectedResult, ObservedResult: review.Result, MatchesExpected: example.ExpectedResult == review.Result}
		report.Results = append(report.Results, result)
		report.Counts.Total++
		switch example.ExpectedResult {
		case "pass":
			report.Counts.ExpectedPass++
			if review.Result == "pass" {
				report.Counts.CorrectPass++
			} else if review.Result == "fail" {
				report.Counts.FalseFail++
			} else {
				report.Counts.Unassessed++
			}
		case "fail":
			report.Counts.ExpectedFail++
			if review.Result == "fail" {
				report.Counts.CorrectFail++
			} else if review.Result == "pass" {
				report.Counts.FalsePass++
			} else {
				report.Counts.Unassessed++
			}
		}
	}
	if len(reviews) != len(seen) || report.Counts.ExpectedPass == 0 || report.Counts.ExpectedFail == 0 {
		return report, fmt.Errorf("%w: suite must contain assessed pass and fail examples only", ErrInvalidSemanticCalibration)
	}
	report.Agreement = float64(report.Counts.CorrectPass+report.Counts.CorrectFail) / float64(report.Counts.Total)
	return report, nil
}

func validateSemanticCalibrationExample(example SemanticCalibrationExample) error {
	if strings.TrimSpace(example.ID) == "" || strings.TrimSpace(example.Output) == "" || strings.TrimSpace(example.ExpectedReason) == "" || strings.TrimSpace(example.InputContext) == "" || !slices.Contains([]string{"pass", "fail"}, example.ExpectedResult) {
		return fmt.Errorf("%w: incomplete example", ErrInvalidSemanticCalibration)
	}
	if digest([]byte(example.Output)) != example.OutputSHA256 {
		return fmt.Errorf("%w: output hash mismatch for %s", ErrInvalidSemanticCalibration, example.ID)
	}
	check := SemanticCheck{ID: example.CriterionID, Criterion: example.Criterion, Severity: example.Severity}
	if strings.TrimSpace(check.ID) == "" || strings.TrimSpace(check.Criterion) == "" || semanticCriterionHash(check) != example.CriterionSHA256 {
		return fmt.Errorf("%w: criterion hash mismatch for %s", ErrInvalidSemanticCalibration, example.ID)
	}
	return nil
}

func validateCalibrationReview(review SemanticCalibrationReview, output string) error {
	if !slices.Contains([]string{"pass", "fail", "unassessed"}, review.Result) || strings.TrimSpace(review.Reason) == "" {
		return fmt.Errorf("%w: result and reason are required", ErrInvalidSemanticCalibration)
	}
	switch review.EvidenceLocation {
	case "present":
		if review.EvidenceQuote == "" || !strings.Contains(output, review.EvidenceQuote) {
			return fmt.Errorf("%w: quote is not present in calibrated output", ErrInvalidSemanticCalibration)
		}
	case "absent":
		if review.EvidenceQuote != "" {
			return fmt.Errorf("%w: absent evidence cannot contain a quote", ErrInvalidSemanticCalibration)
		}
	default:
		return fmt.Errorf("%w: evidence location must be present or absent", ErrInvalidSemanticCalibration)
	}
	return nil
}

func EvaluateSemanticCalibrationFiles(suitePath, assessmentsPath string) (SemanticCalibrationReport, error) {
	var report SemanticCalibrationReport
	suiteData, err := readSemanticCalibrationFile(suitePath)
	if err != nil {
		return report, err
	}
	assessmentsData, err := readSemanticCalibrationFile(assessmentsPath)
	if err != nil {
		return report, err
	}
	var suite SemanticCalibrationSuite
	if err = decodeSemanticCalibrationJSON(suiteData, &suite); err != nil {
		return report, fmt.Errorf("decode calibration suite: %w", err)
	}
	var assessments SemanticCalibrationAssessments
	if err = decodeSemanticCalibrationJSON(assessmentsData, &assessments); err != nil {
		return report, fmt.Errorf("decode calibration assessments: %w", err)
	}
	if assessments.CalibrationSHA256 != digest(suiteData) {
		return report, fmt.Errorf("%w: assessments do not bind the exact calibration suite", ErrInvalidSemanticCalibration)
	}
	report, err = CalibrateSemanticReviewer(suite, assessments)
	if err != nil {
		return report, err
	}
	report.CalibrationSHA256 = digest(suiteData)
	report.AssessmentsSHA256 = digest(assessmentsData)
	return report, nil
}

func readSemanticCalibrationFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxSemanticReviewBytes+1))
	if err = errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if len(data) > maxSemanticReviewBytes {
		return nil, fmt.Errorf("%w: document exceeds size limit", ErrInvalidSemanticCalibration)
	}
	return data, nil
}

func decodeSemanticCalibrationJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing JSON data", ErrInvalidSemanticCalibration)
	}
	return nil
}
