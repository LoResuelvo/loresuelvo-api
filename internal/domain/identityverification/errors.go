package identityverification

import "errors"

var (
	ErrProviderRequired            = errors.New("a registered provider is required")
	ErrInvalidVerification         = errors.New("identity verification is invalid")
	ErrVerificationAlreadyApproved = errors.New("identity verification is already approved")
	ErrVerifierUnavailable         = errors.New("identity verifier is temporarily unavailable")
	ErrVerifierMisconfigured       = errors.New("identity verifier is not configured")
	ErrSessionNotFound             = errors.New("identity verification session does not exist")
)
