package cryptography

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCalendarCredentialCipherFromEnvUsesIndependentKey(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	t.Setenv(calendarCredentialEncryptionKeyEnv, base64.StdEncoding.EncodeToString(key))
	t.Setenv(paymentAccountCredentialEncryptionKeyEnv, "")

	cipher, err := NewCalendarCredentialCipherFromEnv()
	require.NoError(t, err)
	ciphertext, err := cipher.Encrypt("calendar-refresh-token")
	require.NoError(t, err)
	plaintext, err := cipher.Decrypt(ciphertext)

	require.NoError(t, err)
	assert.Equal(t, "calendar-refresh-token", plaintext)
}
