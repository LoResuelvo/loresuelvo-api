package workorder

import "time"

const ConfirmationCodeLength = 4

type ConfirmationCode struct {
	value string
}

func NewConfirmationCode(value string) (ConfirmationCode, error) {
	if len(value) != ConfirmationCodeLength {
		return ConfirmationCode{}, ErrInvalidCompletionAuthorization
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return ConfirmationCode{}, ErrInvalidCompletionAuthorization
		}
	}
	return ConfirmationCode{value: value}, nil
}

func (code ConfirmationCode) String() string {
	return code.value
}

type CompletionAuthorization struct {
	codeCiphertext []byte
	issuedOn       time.Time
}

func NewCompletionAuthorization(codeCiphertext []byte, issuedOn time.Time) (*CompletionAuthorization, error) {
	if len(codeCiphertext) == 0 || issuedOn.IsZero() {
		return nil, ErrInvalidCompletionAuthorization
	}
	return &CompletionAuthorization{
		codeCiphertext: append([]byte(nil), codeCiphertext...),
		issuedOn:       issuedOn.UTC(),
	}, nil
}

func (authorization *CompletionAuthorization) CodeCiphertext() []byte {
	if authorization == nil {
		return nil
	}
	return append([]byte(nil), authorization.codeCiphertext...)
}

func (authorization *CompletionAuthorization) IssuedOn() time.Time {
	if authorization == nil {
		return time.Time{}
	}
	return authorization.issuedOn
}

func (authorization *CompletionAuthorization) valid() bool {
	return authorization != nil && len(authorization.codeCiphertext) > 0 && !authorization.issuedOn.IsZero()
}
