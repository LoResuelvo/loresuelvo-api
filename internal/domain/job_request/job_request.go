package jobrequest

import "strings"

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
)

func OpenStatuses() []Status {
	return []Status{StatusPending, StatusAccepted}
}

type JobRequest struct {
	ID             int
	ConsumerID     int
	ProviderID     int
	ConversationID int
	Title          string
	Description    string
	Status         Status
}

func New(consumerID, providerID int, title, description string) (*JobRequest, error) {
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

	return &JobRequest{
		ConsumerID:  consumerID,
		ProviderID:  providerID,
		Title:       trimmedTitle,
		Description: strings.TrimSpace(description),
		Status:      StatusPending,
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
