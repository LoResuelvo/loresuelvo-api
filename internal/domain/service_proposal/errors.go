package serviceproposal

import "errors"

var (
	ErrProviderRequired      = errors.New("Provider id is required")
	ErrConsumerRequired      = errors.New("Consumer id is required")
	ErrInvalidAmount         = errors.New("Amount must be greater than 0")
	ErrInvalidScheduledOn    = errors.New("Scheduled on must be in the future")
	ErrConversationRequired  = errors.New("Provider and consumer must have an active conversation before creating a service proposal")
	ErrConversationNotActive = errors.New("Conversation is not active")
)
