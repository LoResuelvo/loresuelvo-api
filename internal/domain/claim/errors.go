package claim

import "errors"

var (
	ErrInvalidSubmission     = errors.New("invalid claim submission")
	ErrInvalidIdempotencyKey = errors.New("invalid claim idempotency key")
	ErrInvalidCriteria       = errors.New("invalid claim list criteria")
	ErrForbidden             = errors.New("claimant account is not enabled")
	ErrNotFound              = errors.New("claim resource not found")
	ErrSubmissionKeyConflict = errors.New("claim submission key already used with different content")
	ErrOpenClaimConflict     = errors.New("an unfinished claim already exists for this operation")
	ErrInvalidEvidence       = errors.New("invalid claim evidence")
	ErrPersistenceConflict   = errors.New("claim persistence constraint conflict")
)
