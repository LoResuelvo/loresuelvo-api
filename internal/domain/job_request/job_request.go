package jobrequest

import (
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"

	MaxJobRequestImages = 3
)

func OpenStatuses() []Status {
	return []Status{StatusPending, StatusAccepted}
}

type JobRequest struct {
	ID                 int
	ConsumerID         int
	ProviderID         int
	ConversationID     int
	Title              string
	Description        string
	Status             Status
	SourceAssessmentID *int
	Images             []Image
}

func NewFromAssessment(consumerID, providerID int, assessment conversation.ProblemAssessment) (*JobRequest, error) {
	if !assessment.RequiresProfessional() {
		return nil, ErrAssessmentNotContactable
	}
	jobRequest, err := New(consumerID, providerID, assessment.ProblemTitle, assessment.ProblemDescription, nil)
	if err != nil {
		return nil, err
	}
	assessmentID := assessment.ID
	if assessmentID <= 0 {
		return nil, ErrAssessmentNotContactable
	}
	jobRequest.SourceAssessmentID = &assessmentID
	return jobRequest, nil
}

func New(consumerID, providerID int, title, description string, images []Image) (*JobRequest, error) {
	if consumerID <= 0 {
		return nil, ErrOnlyConsumerCanCreateJobRequest
	}
	if providerID <= 0 {
		return nil, ErrProviderRequired
	}

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return nil, ErrTitleRequired
	}
	if len(images) > MaxJobRequestImages {
		return nil, ErrJobRequestImageNotAvailable
	}

	return &JobRequest{
		ConsumerID:  consumerID,
		ProviderID:  providerID,
		Title:       trimmedTitle,
		Description: strings.TrimSpace(description),
		Status:      StatusPending,
		Images:      append([]Image(nil), images...),
	}, nil
}

func (jobRequest JobRequest) CanBeAcceptedBy(providerID int) bool {
	return jobRequest.ProviderID == providerID
}

func (jobRequest *JobRequest) Accept(providerID int) error {
	if !jobRequest.CanBeAcceptedBy(providerID) {
		return ErrOnlyAssignedProviderCanAcceptJobRequest
	}
	if jobRequest.Status != StatusPending {
		return ErrOnlyPendingJobRequestCanBeAccepted
	}

	jobRequest.Status = StatusAccepted
	return nil
}
