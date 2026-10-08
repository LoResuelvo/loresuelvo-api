package claim

import "errors"

var (
	ErrAdministrationRecordConflict = errors.New("claim administration record persistence conflict")
	ErrInvalidResolution            = errors.New("invalid claim resolution")
	ErrInvalidTransition            = errors.New("claim state does not allow this action")
	ErrAdministrationKeyConflict    = errors.New("claim administration key already used with different content")
	ErrInvalidSubmission            = errors.New("invalid claim submission")
	ErrInvalidIdempotencyKey        = errors.New("invalid claim idempotency key")
	ErrInvalidCriteria              = errors.New("invalid claim list criteria")
	ErrForbidden                    = errors.New("claimant account is not enabled")
	ErrNotFound                     = errors.New("claim resource not found")
	ErrSubmissionKeyConflict        = errors.New("claim submission key already used with different content")
	ErrOpenClaimConflict            = errors.New("an unfinished claim already exists for this operation")
	ErrEvidenceAccessUnavailable    = errors.New("claim evidence access is unavailable")
	ErrInvalidEvidence              = errors.New("invalid claim evidence")
	ErrPersistenceConflict          = errors.New("claim persistence constraint conflict")
)
