package calendarconnection

type CredentialProtector interface {
	Encrypt(plaintext string) ([]byte, error)
	Decrypt(ciphertext []byte) (string, error)
}

type SecretGenerator interface {
	Generate() (string, error)
}
