package cryptography

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var ErrInvalidEncryptionKey = errors.New("credential encryption key must be a base64-encoded 32-byte key")

type AESGCMCipher struct {
	aead cipher.AEAD
}

func NewAESGCMCipher(encodedKey string) (*AESGCMCipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidEncryptionKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating credential AEAD: %w", err)
	}
	return &AESGCMCipher{aead: aead}, nil
}

func (cipher *AESGCMCipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, cipher.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating credential nonce: %w", err)
	}
	return cipher.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (cipher *AESGCMCipher) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := cipher.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("invalid encrypted credential")
	}
	nonce, encryptedValue := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := cipher.aead.Open(nil, nonce, encryptedValue, nil)
	if err != nil {
		return "", errors.New("invalid encrypted credential")
	}
	return string(plaintext), nil
}
