package workorder

import (
	"strings"
	"time"
)

const maxCompletionReportImages = 3

// CompletionReport records the evidence submitted for a completed work order.
// Its fields are immutable through the aggregate; only its identity can be
// assigned by persistence after a new report has been saved.
type CompletionReport struct {
	id           int
	description  string
	imageFileIDs []string
	reportedOn   time.Time
}

func NewCompletionReport(description string, imageFileIDs []string, reportedOn time.Time) (*CompletionReport, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, ErrCompletionReportDescriptionRequired
	}
	if len(imageFileIDs) < 1 || len(imageFileIDs) > maxCompletionReportImages {
		return nil, ErrCompletionReportImageCount
	}

	normalizedImageFileIDs := make([]string, len(imageFileIDs))
	seen := make(map[string]struct{}, len(imageFileIDs))
	for index, imageFileID := range imageFileIDs {
		imageFileID = strings.TrimSpace(imageFileID)
		if imageFileID == "" {
			return nil, ErrCompletionReportImageRequired
		}
		if _, exists := seen[imageFileID]; exists {
			return nil, ErrCompletionReportDuplicateImage
		}
		seen[imageFileID] = struct{}{}
		normalizedImageFileIDs[index] = imageFileID
	}
	if reportedOn.IsZero() {
		return nil, ErrCompletionReportReportedOnRequired
	}

	return &CompletionReport{
		description:  description,
		imageFileIDs: normalizedImageFileIDs,
		reportedOn:   reportedOn.UTC(),
	}, nil
}

func (report *CompletionReport) ID() int {
	if report == nil {
		return 0
	}
	return report.id
}

// SetID is a technical identity setter used by persistence after INSERT.
func (report *CompletionReport) SetID(id int) {
	if report == nil {
		return
	}
	report.id = id
}

func (report *CompletionReport) Description() string {
	if report == nil {
		return ""
	}
	return report.description
}

func (report *CompletionReport) ImageFileIDs() []string {
	if report == nil {
		return nil
	}
	return append([]string(nil), report.imageFileIDs...)
}

func (report *CompletionReport) ReportedOn() time.Time {
	if report == nil {
		return time.Time{}
	}
	return report.reportedOn
}
