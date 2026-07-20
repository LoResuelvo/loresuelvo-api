package paymentaccount

import "errors"

var (
	ErrOnlyProvidersCanConnect       = errors.New("only providers can connect payment accounts")
	ErrPaymentProviderRequired       = errors.New("payment provider is required")
	ErrPaymentProviderMismatch       = errors.New("authorization attempt belongs to a different payment provider")
	ErrAuthorizationStateRequired    = errors.New("authorization state is required")
	ErrAuthorizationCodeRequired     = errors.New("authorization code is required")
	ErrAuthorizationCodeUnusable     = errors.New("authorization code cannot be used")
	ErrAuthorizationGrantUnavailable = errors.New("payment account authorization grant is unavailable")
	ErrAuthorizationAttemptExpired   = errors.New("authorization attempt has expired")
	ErrAuthorizationAttemptNotFound  = errors.New("authorization attempt does not exist")
	ErrConnectionNotFound            = errors.New("payment account connection does not exist")
	ErrAlreadyConnected              = errors.New("payment account is already connected")
	ErrExternalAccountAlreadyLinked  = errors.New("payment account is already linked to another provider")
	ErrExternalAccountIDRequired     = errors.New("external payment account id is required")
	ErrAccessTokenRequired           = errors.New("payment account access token is required")
)
