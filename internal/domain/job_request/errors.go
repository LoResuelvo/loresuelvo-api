package jobrequest

import "errors"

var ErrProviderDoesNotExist = errors.New("Provider does not exist")

var ErrProviderRequired = errors.New("Provider id is required")

var ErrTitleRequired = errors.New("Title is required")

var ErrOnlyConsumerCanCreateJobRequest = errors.New("Only consumers can create job requests")

var ErrAlreadyExists = errors.New("Job request already exists")

var ErrJobRequestNotFound = errors.New("Job request not found")

var ErrOnlyAssignedProviderCanAcceptJobRequest = errors.New("Only the assigned provider can accept job requests")
