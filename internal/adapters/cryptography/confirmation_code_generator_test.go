package cryptography_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/cryptography"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureConfirmationCodeGeneratorProducesFourDigits(t *testing.T) {
	generator := cryptography.NewSecureConfirmationCodeGenerator()

	for range 100 {
		code, err := generator.Generate()
		require.NoError(t, err)
		assert.Len(t, code.String(), workorder.ConfirmationCodeLength)
		_, err = workorder.NewConfirmationCode(code.String())
		assert.NoError(t, err)
	}
}
