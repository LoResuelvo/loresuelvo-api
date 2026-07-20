package cryptography_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/cryptography"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESGCMCipherEncryptsAndDecryptsCredential(t *testing.T) {
	encodedKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	cipher, err := cryptography.NewAESGCMCipher(encodedKey)
	require.NoError(t, err)

	ciphertext, err := cipher.Encrypt("sensitive-token")

	require.NoError(t, err)
	assert.NotContains(t, string(ciphertext), "sensitive-token")
	plaintext, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "sensitive-token", plaintext)
}

func TestAESGCMCipherRejectsInvalidKey(t *testing.T) {
	_, err := cryptography.NewAESGCMCipher("not-a-base64-encoded-key")

	assert.Error(t, err)
}

func TestNewCredentialCipherFromEnvOwnsCredentialEncryptionConfiguration(t *testing.T) {
	encodedKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	t.Setenv("PAYMENT_ACCOUNT_CREDENTIAL_ENCRYPTION_KEY", encodedKey)

	cipher, err := cryptography.NewCredentialCipherFromEnv()

	require.NoError(t, err)
	ciphertext, err := cipher.Encrypt("sensitive-token")
	require.NoError(t, err)
	plaintext, err := cipher.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "sensitive-token", plaintext)
}
