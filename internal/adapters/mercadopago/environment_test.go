package mercadopago_test

import (
	"testing"

	sharedmercadopago "github.com/LoResuelvo/loresuelvo-api/internal/adapters/mercadopago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvironmentAcceptsSupportedEnvironments(t *testing.T) {
	for _, expected := range []sharedmercadopago.Environment{
		sharedmercadopago.EnvironmentSandbox,
		sharedmercadopago.EnvironmentProduction,
	} {
		t.Run(string(expected), func(t *testing.T) {
			environment, err := sharedmercadopago.ParseEnvironment(string(expected))

			require.NoError(t, err)
			assert.Equal(t, expected, environment)
		})
	}
}

func TestEnvironmentFromEnvRejectsMissingOrUnsupportedValues(t *testing.T) {
	for _, value := range []string{"", "test", "SANDBOX"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(sharedmercadopago.EnvironmentVariable, value)

			_, err := sharedmercadopago.EnvironmentFromEnv()

			assert.ErrorIs(t, err, sharedmercadopago.ErrInvalidEnvironment)
		})
	}
}
