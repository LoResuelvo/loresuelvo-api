package serviceproposal

import "errors"

var (
	ErrProviderRequired = errors.New("Provider id is required")
	ErrConsumerRequired = errors.New("Consumer id is required")
	ErrInvalidAmount    = errors.New("Amount must be greater than 0")
)
