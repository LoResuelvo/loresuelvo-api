package identityverification

import "errors"

var (
	ErrProviderRequired            = errors.New("a registered provider is required")
	ErrVerificationAlreadyApproved = errors.New("identity verification is already approved")
	ErrVerificationInReview        = errors.New("identity verification is under manual review")
	ErrSessionNotFound             = errors.New("identity verification session does not exist")
	ErrInvalidResult               = errors.New("identity verification result is invalid")
	ErrVerifierUnavailable         = errors.New("identity verifier is temporarily unavailable")
	ErrVerifierMisconfigured       = errors.New("identity verifier is not configured")
)
