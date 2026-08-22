package calendarconnection

import "time"

type Authorization struct {
	URL   string
	State string
}

type AuthorizationAttempt struct {
	ID                     int
	UserID                 int
	StateDigest            []byte
	CodeVerifierCiphertext []byte
	ExpiresOn              time.Time
	ConsumedOn             *time.Time
}

func NewAuthorizationAttempt(userID int, stateDigest, codeVerifierCiphertext []byte, expiresOn time.Time) (*AuthorizationAttempt, error) {
	if userID <= 0 {
		return nil, ErrUserIDRequired
	}
	if len(stateDigest) == 0 {
		return nil, ErrAuthorizationStateRequired
	}
	if len(codeVerifierCiphertext) == 0 {
		return nil, ErrAuthorizationCodeRequired
	}
	return &AuthorizationAttempt{
		UserID:                 userID,
		StateDigest:            append([]byte(nil), stateDigest...),
		CodeVerifierCiphertext: append([]byte(nil), codeVerifierCiphertext...),
		ExpiresOn:              expiresOn.UTC(),
	}, nil
}

func RehydrateAuthorizationAttempt(id, userID int, stateDigest, codeVerifierCiphertext []byte, expiresOn time.Time, consumedOn *time.Time) *AuthorizationAttempt {
	var consumedAt *time.Time
	if consumedOn != nil {
		value := consumedOn.UTC()
		consumedAt = &value
	}
	return &AuthorizationAttempt{
		ID:                     id,
		UserID:                 userID,
		StateDigest:            append([]byte(nil), stateDigest...),
		CodeVerifierCiphertext: append([]byte(nil), codeVerifierCiphertext...),
		ExpiresOn:              expiresOn.UTC(),
		ConsumedOn:             consumedAt,
	}
}

func (attempt *AuthorizationAttempt) IsExpired(now time.Time) bool {
	return !now.UTC().Before(attempt.ExpiresOn)
}

func (attempt *AuthorizationAttempt) IsConsumed() bool {
	return attempt.ConsumedOn != nil
}
