package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAdminSeedAuthID  = "test-admin-seed-identity"
	testAdminSeedEmail   = "admin-seed@example.invalid"
	testAdminSeedName    = "Seed"
	testAdminSeedSurname = "Administrator"
)

func TestAdminSeedConfigFromEnvRequiresEveryValue(t *testing.T) {
	variables := []string{adminSeedAuthIDEnv, adminSeedEmailEnv, adminSeedNameEnv, adminSeedSurnameEnv}
	for _, missingVariable := range variables {
		t.Run(missingVariable, func(t *testing.T) {
			setValidAdminSeedEnvironment(t)
			t.Setenv(missingVariable, " \t ")

			_, err := adminSeedConfigFromEnv()

			require.ErrorIs(t, err, ErrInvalidAdminSeedConfiguration)
			assert.ErrorContains(t, err, missingVariable)
		})
	}
}

func TestAdminSeedConfigFromEnvTrimsValues(t *testing.T) {
	setValidAdminSeedEnvironment(t)
	t.Setenv(adminSeedAuthIDEnv, "  "+testAdminSeedAuthID+"  ")
	t.Setenv(adminSeedEmailEnv, "  "+testAdminSeedEmail+"  ")
	t.Setenv(adminSeedNameEnv, "  "+testAdminSeedName+"  ")
	t.Setenv(adminSeedSurnameEnv, "  "+testAdminSeedSurname+"  ")

	config, err := adminSeedConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, testAdminSeedAuthID, config.authID)
	assert.Equal(t, testAdminSeedEmail, config.email)
	assert.Equal(t, testAdminSeedName, config.name)
	assert.Equal(t, testAdminSeedSurname, config.surname)
}

func TestAdminSeedConfigFromEnvRejectsValuesBeyondSchemaLimits(t *testing.T) {
	tests := []struct {
		variable string
		value    string
	}{
		{variable: adminSeedAuthIDEnv, value: strings.Repeat("a", adminSeedIdentityMaxLength+1)},
		{variable: adminSeedEmailEnv, value: strings.Repeat("a", adminSeedIdentityMaxLength-14) + "@example.invalid"},
		{variable: adminSeedNameEnv, value: strings.Repeat("a", adminSeedNameMaxLength+1)},
		{variable: adminSeedSurnameEnv, value: strings.Repeat("a", adminSeedNameMaxLength+1)},
	}
	for _, test := range tests {
		t.Run(test.variable, func(t *testing.T) {
			setValidAdminSeedEnvironment(t)
			t.Setenv(test.variable, test.value)

			_, err := adminSeedConfigFromEnv()

			require.ErrorIs(t, err, ErrInvalidAdminSeedConfiguration)
			assert.ErrorContains(t, err, test.variable)
			assert.NotContains(t, err.Error(), test.value)
		})
	}
}

func TestSeedAdminFromEnvRejectsInvalidEmailWithoutDisclosingConfiguredValues(t *testing.T) {
	setValidAdminSeedEnvironment(t)
	invalidEmail := "sensitive-invalid-email"
	t.Setenv(adminSeedEmailEnv, invalidEmail)

	err := SeedAdminFromEnv(context.Background(), nil)

	require.ErrorIs(t, err, ErrInvalidAdminSeedConfiguration)
	assert.ErrorContains(t, err, adminSeedEmailEnv)
	assert.NotContains(t, err.Error(), invalidEmail)
	assert.NotContains(t, err.Error(), testAdminSeedAuthID)
}

func TestApplicationSeedValidationRunsBeforeOptionalDefaultSeeds(t *testing.T) {
	setValidAdminSeedEnvironment(t)
	t.Setenv(adminSeedAuthIDEnv, "")
	t.Setenv("SEEDS_ENABLED", "true")
	t.Setenv("SEEDS_FILE", t.TempDir()+"/malformed.yaml")

	err := seedApplicationDataFromEnv(context.Background(), nil)

	require.ErrorIs(t, err, ErrInvalidAdminSeedConfiguration)
	assert.NotContains(t, err.Error(), "default")
}

func setValidAdminSeedEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(adminSeedAuthIDEnv, testAdminSeedAuthID)
	t.Setenv(adminSeedEmailEnv, testAdminSeedEmail)
	t.Setenv(adminSeedNameEnv, testAdminSeedName)
	t.Setenv(adminSeedSurnameEnv, testAdminSeedSurname)
}
