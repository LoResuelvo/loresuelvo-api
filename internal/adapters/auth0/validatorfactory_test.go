package auth0

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidatorFromEnvUsesFakeValidatorInDevWhenCredentialsAreMissing(t *testing.T) {
	t.Setenv("ENVIRONMENT", "dev")
	t.Setenv("AUTH0_DOMAIN", "")
	t.Setenv("AUTH0_AUDIENCE", "")

	output := captureStdout(t, func() {
		jwtValidator, err := NewValidatorFromEnv()
		require.NoError(t, err)
		require.NotNil(t, jwtValidator)

		claims, err := jwtValidator.ValidateToken(context.Background(), NewTokenBuilder().BuildToken(devSwaggerAuthID, nil))
		require.NoError(t, err)
		validatedClaims := claims.(*validator.ValidatedClaims)
		assert.Equal(t, devSwaggerAuthID, validatedClaims.RegisteredClaims.Subject)
	})

	assert.Contains(t, output, "using fake JWT validator")
	assert.Contains(t, output, "Swagger dev auth_id: "+devSwaggerAuthID)
	assert.Contains(t, output, "Swagger dev Bearer token")
}

func TestNewValidatorFromEnvRejectsMissingCredentialsOutsideDev(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("AUTH0_DOMAIN", "")
	t.Setenv("AUTH0_AUDIENCE", "")

	jwtValidator, err := NewValidatorFromEnv()

	assert.Nil(t, jwtValidator)
	assert.ErrorContains(t, err, "missing AUTH0_DOMAIN or AUTH0_AUDIENCE")
}

func captureStdout(t *testing.T, action func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	action()
	require.NoError(t, writer.Close())
	os.Stdout = originalStdout

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	require.NoError(t, err)

	return strings.TrimSpace(buffer.String())
}
