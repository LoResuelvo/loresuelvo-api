package bootstrap

import (
	"errors"
	"testing"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/stretchr/testify/assert"
)

func TestParsePaymentsDemoModeAcceptsRecognisedValues(t *testing.T) {
	cases := []struct {
		raw      string
		expected bool
	}{
		{raw: "", expected: false},
		{raw: "false", expected: false},
		{raw: "FALSE", expected: false},
		{raw: "0", expected: false},
		{raw: "true", expected: true},
		{raw: "TRUE", expected: true},
		{raw: "  true  ", expected: true},
		{raw: "1", expected: true},
	}
	for _, testCase := range cases {
		got, err := ParsePaymentsDemoMode(testCase.raw)
		assert.NoError(t, err, "input %q should not error", testCase.raw)
		assert.Equal(t, testCase.expected, got, "input %q", testCase.raw)
	}
}

func TestParsePaymentsDemoModeRejectsUnknownValues(t *testing.T) {
	for _, raw := range []string{"yes", "on", "enabled", "2", "tru e"} {
		_, err := ParsePaymentsDemoMode(raw)
		assert.Error(t, err, "input %q should error", raw)
	}
}

func TestValidatePaymentsDemoModeAllowsDevAndTest(t *testing.T) {
	assert.NoError(t,
		ValidatePaymentsDemoMode(true, httpadapter.DevelopmentEnvironment),
		"demo mode must be allowed in development")
	assert.NoError(t,
		ValidatePaymentsDemoMode(true, httpadapter.TestEnvironment),
		"demo mode must be allowed in test")
}

func TestValidatePaymentsDemoModeRejectsStagingAndProduction(t *testing.T) {
	err := ValidatePaymentsDemoMode(true, httpadapter.StagingEnvironment)
	assert.Error(t, err)
	assert.True(t,
		errors.Is(err, ErrPaymentsDemoModeMisconfigured),
		"staging must surface the misconfiguration sentinel")

	err = ValidatePaymentsDemoMode(true, httpadapter.ProductionEnvironment)
	assert.Error(t, err)
	assert.True(t,
		errors.Is(err, ErrPaymentsDemoModeMisconfigured),
		"production must surface the misconfiguration sentinel")
}

func TestValidatePaymentsDemoModeSkipsCheckWhenDisabled(t *testing.T) {
	assert.NoError(t,
		ValidatePaymentsDemoMode(false, httpadapter.ProductionEnvironment),
		"the validator must not refuse the combination when demo mode is disabled")
	assert.NoError(t,
		ValidatePaymentsDemoMode(false, ""),
		"the validator must not refuse the combination when demo mode is disabled, regardless of environment value")
}
