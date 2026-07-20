package paymentaccount

import "time"

type Authorization struct {
	URL   string
	State string
}

type AuthorizationAttempt struct {
	ID                     int
	ProviderID             int
	PaymentProvider        PaymentProvider
	StateDigest            []byte
	CodeVerifierCiphertext []byte
	ExpiresOn              time.Time
}

func NewAuthorizationAttempt(providerID int, paymentProvider PaymentProvider, stateDigest, codeVerifierCiphertext []byte, expiresOn time.Time) *AuthorizationAttempt {
	return &AuthorizationAttempt{
		ProviderID:             providerID,
		PaymentProvider:        paymentProvider,
		StateDigest:            append([]byte(nil), stateDigest...),
		CodeVerifierCiphertext: append([]byte(nil), codeVerifierCiphertext...),
		ExpiresOn:              expiresOn.UTC(),
	}
}

func (attempt *AuthorizationAttempt) IsExpired(now time.Time) bool {
	return !now.UTC().Before(attempt.ExpiresOn)
}
