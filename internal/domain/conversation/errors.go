package conversation

import "errors"

var ErrProviderDoesNotExist = errors.New("Provider does not exist")

var ErrProviderRequired = errors.New("Provider id is required")

var ErrMessageRequired = errors.New("Message is required")

var ErrOnlyConsumerCanStartWorkRequest = errors.New("Only consumers can start work requests")

var ErrAlreadyExists = errors.New("Conversation already exists")
