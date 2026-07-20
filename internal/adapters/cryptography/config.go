package cryptography

import (
	"os"
	"strings"
)

const paymentAccountCredentialEncryptionKeyEnv = "PAYMENT_ACCOUNT_CREDENTIAL_ENCRYPTION_KEY"

func NewCredentialCipherFromEnv() (*AESGCMCipher, error) {
	encodedKey := strings.TrimSpace(os.Getenv(paymentAccountCredentialEncryptionKeyEnv))
	return NewAESGCMCipher(encodedKey)
}
