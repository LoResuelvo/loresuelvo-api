package cryptography

import (
	"os"
	"strings"
)

const calendarCredentialEncryptionKeyEnv = "GOOGLE_CALENDAR_CREDENTIAL_ENCRYPTION_KEY"

func NewCalendarCredentialCipherFromEnv() (*AESGCMCipher, error) {
	encodedKey := strings.TrimSpace(os.Getenv(calendarCredentialEncryptionKeyEnv))
	return NewAESGCMCipher(encodedKey)
}
