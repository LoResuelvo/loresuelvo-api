package jobrequest

import "strings"

type JobRequest struct {
	ID             int
	ConsumerID     int
	ProviderID     int
	ConversationID int
	Title          string
	Description    string
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
	}, nil
}

func (jobRequest JobRequest) CanBeAcceptedBy(providerID int) bool {
	return jobRequest.ProviderID == providerID
}
