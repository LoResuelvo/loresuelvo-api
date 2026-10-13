package evals

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseEvaluationAPIKeysRejectsEmptyOrDuplicateWithoutEchoingValues(t *testing.T) {
	for _, value := range []string{"", "key-a,", ",key-a", "key-a, key-a"} {
		_, err := ParseEvaluationAPIKeys(value)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "key-a")
	}
	keys, err := ParseEvaluationAPIKeys(" key-a, key-b ")
	require.NoError(t, err)
	require.Equal(t, []string{"key-a", "key-b"}, keys)
	require.Equal(t, "[REDACTED] [REDACTED]", CredentialsRedactor(keys)("key-a key-b"))
}

func TestCredentialsRedactorCoversEscapedAndOverlappingKeys(t *testing.T) {
	redact := CredentialsRedactor([]string{"short-key", "short-key-long", `quoted"key`})
	require.Equal(t, `[REDACTED] [REDACTED] [REDACTED]`, redact(`short-key-long quoted"key quoted\"key`))
}
