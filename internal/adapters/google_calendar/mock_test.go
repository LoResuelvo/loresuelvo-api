package googlecalendar

import "github.com/stretchr/testify/mock"

type credentialProtectorMock struct{ mock.Mock }

func (m *credentialProtectorMock) Encrypt(plaintext string) ([]byte, error) {
	args := m.Called(plaintext)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *credentialProtectorMock) Decrypt(ciphertext []byte) (string, error) {
	args := m.Called(ciphertext)
	return args.String(0), args.Error(1)
}
