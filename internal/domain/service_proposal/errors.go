package serviceproposal

import "errors"

var ErrProviderRequired = errors.New("Provider id is required")

var ErrConsumerRequired = errors.New("Consumer id is required")
