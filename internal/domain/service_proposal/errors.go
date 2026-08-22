package serviceproposal

import "errors"

var (
	ErrProviderRequired                 = errors.New("Provider id is required")
	ErrConsumerRequired                 = errors.New("Consumer id is required")
	ErrInvalidAmount                    = errors.New("Amount must be greater than 0")
	ErrEstimatedDurationRequired        = errors.New("Estimated duration is required")
	ErrInvalidEstimatedDuration         = errors.New("Estimated duration must be between 15 and 1440 minutes")
	ErrInvalidScheduledOn               = errors.New("Scheduled on must be in the future")
	ErrInsufficientBookingLeadTime      = errors.New("Scheduled on must be more than 24 hours in the future")
	ErrConversationRequired             = errors.New("Provider and consumer must have an active conversation before creating a service proposal")
	ErrConversationNotActive            = errors.New("Conversation is not active")
	ErrDoesNotExist                     = errors.New("Service proposal does not exist")
	ErrOnlyRecipientCanAccept           = errors.New("Only the recipient consumer can accept the service proposal")
	ErrOnlyPendingCanBeAccepted         = errors.New("Only pending service proposals can be accepted")
	ErrServiceProposalExpired           = errors.New("Service proposal has expired")
	ErrOnlyParticipantCanView           = errors.New("Only a participant can view the service proposal")
	ErrPaymentAccountConnectionRequired = errors.New("A connected payment account is required before creating a service proposal")
)
