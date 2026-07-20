package cryptography

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type SecureSecretGenerator struct{}

func NewSecureSecretGenerator() *SecureSecretGenerator {
	return &SecureSecretGenerator{}
}

func (generator *SecureSecretGenerator) Generate() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generating secure random value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
