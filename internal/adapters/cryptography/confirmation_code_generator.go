package cryptography

import (
	"crypto/rand"
	"fmt"
	"math/big"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

const confirmationCodeUpperBound = 10_000

type SecureConfirmationCodeGenerator struct{}

func NewSecureConfirmationCodeGenerator() *SecureConfirmationCodeGenerator {
	return &SecureConfirmationCodeGenerator{}
}

func (*SecureConfirmationCodeGenerator) Generate() (workorder.ConfirmationCode, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(confirmationCodeUpperBound))
	if err != nil {
		return workorder.ConfirmationCode{}, fmt.Errorf("generating secure confirmation code: %w", err)
	}
	code, err := workorder.NewConfirmationCode(fmt.Sprintf("%04d", value.Int64()))
	if err != nil {
		return workorder.ConfirmationCode{}, fmt.Errorf("constructing generated confirmation code: %w", err)
	}
	return code, nil
}
