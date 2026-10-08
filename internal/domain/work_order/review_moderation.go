package workorder

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
)

type ModerationInput struct {
	Action          string
	Category        string
	Reason          string
	ExpectedVersion int
	ReportID        int
}

func (input ModerationInput) Normalize() (ModerationInput, error) {
	reason, err := audit.NewReason(input.Reason)
	if err != nil || input.ExpectedVersion <= 0 || input.ReportID < 0 {
		return ModerationInput{}, ErrInvalidReviewModeration
	}
	input.Reason = reason.Text()
	switch input.Action {
	case "hide":
		if !validReviewReportCategory(input.Category) {
			return ModerationInput{}, ErrInvalidReviewModeration
		}
	case "unhide":
		if input.Category != "" || input.ReportID != 0 {
			return ModerationInput{}, ErrInvalidReviewModeration
		}
	case "dismiss_reports":
		if input.ReportID <= 0 || input.Category != "" {
			return ModerationInput{}, ErrInvalidReviewModeration
		}
	default:
		return ModerationInput{}, ErrInvalidReviewModeration
	}
	return input, nil
}

type ReviewDecision struct {
	id, workOrderID, operatorID, reportID, previousHideID int
	action, category, reason                              string
	createdOn                                             time.Time
}

func (d *ReviewDecision) ID() int              { return d.id }
func (d *ReviewDecision) SetID(id int)         { d.id = id }
func (d *ReviewDecision) WorkOrderID() int     { return d.workOrderID }
func (d *ReviewDecision) OperatorID() int      { return d.operatorID }
func (d *ReviewDecision) ReportID() int        { return d.reportID }
func (d *ReviewDecision) PreviousHideID() int  { return d.previousHideID }
func (d *ReviewDecision) Action() string       { return d.action }
func (d *ReviewDecision) Category() string     { return d.category }
func (d *ReviewDecision) Reason() string       { return d.reason }
func (d *ReviewDecision) CreatedOn() time.Time { return d.createdOn }
func RestoreReviewDecision(id, orderID, operatorID, reportID, previousHideID int, action, category, reason string, at time.Time) (*ReviewDecision, error) {
	if id <= 0 || orderID <= 0 || operatorID <= 0 || at.IsZero() {
		return nil, ErrInvalidReviewModeration
	}
	if _, err := (ModerationInput{Action: action, Category: category, Reason: reason, ExpectedVersion: 1, ReportID: reportID}).Normalize(); err != nil {
		return nil, err
	}
	return &ReviewDecision{id: id, workOrderID: orderID, operatorID: operatorID, reportID: reportID, previousHideID: previousHideID, action: action, category: category, reason: reason, createdOn: at.UTC()}, nil
}
func (review *Review) Version() int          { return review.version }
func (review *Review) HidingDecisionID() int { return review.hidingDecisionID }

func (review *Review) Moderate(orderID int, input ModerationInput, report *ReviewReport, operatorID int, at time.Time) (*ReviewDecision, error) {
	input, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	if operatorID <= 0 || orderID <= 0 || at.IsZero() {
		return nil, ErrInvalidReviewModeration
	}
	if review.version != input.ExpectedVersion {
		return nil, ErrReviewModerationConflict
	}
	if input.ReportID > 0 {
		if err := report.ValidatePendingForReview(orderID, input.ReportID); err != nil {
			return nil, err
		}
	}
	if input.Action == "unhide" && (review.visible || review.hidingDecisionID <= 0) {
		return nil, ErrReviewModerationConflict
	}
	if input.Action == "hide" && !review.visible && input.ReportID == 0 {
		return nil, ErrReviewModerationConflict
	}
	decision := &ReviewDecision{workOrderID: orderID, operatorID: operatorID, reportID: input.ReportID, action: input.Action, category: input.Category, reason: input.Reason, createdOn: at.UTC().Truncate(time.Microsecond)}
	switch input.Action {
	case "hide":
		if input.ReportID > 0 {
			if err := report.Uphold(orderID, input.ReportID); err != nil {
				return nil, err
			}
		}
		review.visible = false
	case "unhide":
		review.visible = true
		decision.previousHideID = review.hidingDecisionID
		review.hidingDecisionID = 0
	case "dismiss_reports":
		if err := report.Dismiss(orderID, input.ReportID); err != nil {
			return nil, err
		}
	}
	review.version++
	return decision, nil
}

// AssociateDecision connects the generated identity to the current hiding decision.
func (review *Review) AssociateDecision(decision *ReviewDecision) {
	if decision.Action() == "hide" {
		review.hidingDecisionID = decision.ID()
	}
}
func (review *Review) NewReport(orderID, providerID int, category, explanation string, at time.Time) (*ReviewReport, error) {
	if !review.Visible() {
		return nil, ErrReviewNotAvailable
	}
	return NewReviewReport(orderID, providerID, category, explanation, at)
}
