package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapturedBodyAttributesRedactsSensitiveIdentityFieldsRecursively(t *testing.T) {
	capture := &limitedBodyCapture{}
	original := []byte(`{
		"email":"sensitive@example.invalid",
		"profile":{"Auth-ID":"auth0|redact-me","user_id":42},
		"metadata":[{"ADMIN-SEED-EMAIL":"seed@example.invalid"},{"admin.seed.auth.id":"auth0|seed-redact-me"}]
	}`)
	_, err := capture.Write(original)
	require.NoError(t, err)

	attributes := capturedBodyAttributes("request", capture, "application/json")
	require.Len(t, attributes, 2)
	require.Equal(t, "request_body", attributes[0])

	redacted, ok := attributes[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", redacted["email"])
	profile, ok := redacted["profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", profile["Auth-ID"])
	require.Equal(t, json.Number("42"), profile["user_id"])
	metadata, ok := redacted["metadata"].([]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", metadata[0].(map[string]any)["ADMIN-SEED-EMAIL"])
	require.Equal(t, "[REDACTED]", metadata[1].(map[string]any)["admin.seed.auth.id"])
}

func TestRedactJSONValueDoesNotMutateOriginalValue(t *testing.T) {
	original := map[string]any{
		"email": "sensitive@example.invalid",
		"nested": map[string]any{
			"auth_id": "auth0|redact-me",
			"user_id": float64(42),
		},
	}

	redacted := redactJSONValue(original)

	require.Equal(t, map[string]any{
		"email": "sensitive@example.invalid",
		"nested": map[string]any{
			"auth_id": "auth0|redact-me",
			"user_id": float64(42),
		},
	}, original)
	require.Equal(t, map[string]any{
		"email": "[REDACTED]",
		"nested": map[string]any{
			"auth_id": "[REDACTED]",
			"user_id": float64(42),
		},
	}, redacted)
}
