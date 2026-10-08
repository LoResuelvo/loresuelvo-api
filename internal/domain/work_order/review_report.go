package workorder

import (
	"strings"
	"time"
	"unicode/utf8"
)

// ReviewReport records an immutable request to review a consumer's assessment.
type ReviewReport struct {
	id, workOrderID, reporterID   int
	category, explanation, status string
	createdOn                     time.Time
}

func NewReviewReport(orderID, reporterID int, category, explanation string, createdOn time.Time) (*ReviewReport, error) {
	explanation = strings.TrimSpace(explanation)
	if !validReviewReportCategory(category) {
		return nil, ErrInvalidReviewReport
	}
	if orderID <= 0 || reporterID <= 0 || createdOn.IsZero() || !utf8.ValidString(explanation) || strings.ContainsRune(explanation, 0) || utf8.RuneCountInString(explanation) > 500 {
		return nil, ErrInvalidReviewReport
	}
	return &ReviewReport{
		workOrderID: orderID,
		reporterID:  reporterID,
		category:    category,
		explanation: explanation,
		status:      "pending",
		createdOn:   createdOn.UTC(),
	}, nil
}

// RestoreReviewReport hydrates persisted attention state without changing receipt data.
func RestoreReviewReport(id, orderID, reporterID int, category, explanation, status string, createdOn time.Time) (*ReviewReport, error) {
	report, err := NewReviewReport(orderID, reporterID, category, explanation, createdOn)
	if err != nil {
		return nil, err
	}
	if id <= 0 || (status != "pending" && status != "upheld" && status != "dismissed") {
		return nil, ErrInvalidReviewReport
	}
	report.id = id
	report.status = status
	return report, nil
}
func (r *ReviewReport) ID() int              { return r.id }
func (r *ReviewReport) SetID(id int)         { r.id = id }
func (r *ReviewReport) WorkOrderID() int     { return r.workOrderID }
func (r *ReviewReport) ReporterID() int      { return r.reporterID }
func (r *ReviewReport) Category() string     { return r.category }
func (r *ReviewReport) Explanation() string  { return r.explanation }
func (r *ReviewReport) Status() string       { return r.status }
func (r *ReviewReport) CreatedOn() time.Time { return r.createdOn }

func validReviewReportCategory(category string) bool {
	switch category {
	case "abusive_language", "personal_data", "spam_advertising", "unrelated_content":
		return true
	default:
		return false
	}
}

func (report *ReviewReport) ValidatePendingForReview(orderID, reportID int) error {
	if report == nil || report.id != reportID || report.workOrderID != orderID || report.status != "pending" {
		return ErrInvalidReviewModeration
	}
	return nil
}
func (report *ReviewReport) Uphold(orderID, reportID int) error {
	if err := report.ValidatePendingForReview(orderID, reportID); err != nil {
		return err
	}
	report.status = "upheld"
	return nil
}
func (report *ReviewReport) Dismiss(orderID, reportID int) error {
	if err := report.ValidatePendingForReview(orderID, reportID); err != nil {
		return err
	}
	report.status = "dismissed"
	return nil
}
