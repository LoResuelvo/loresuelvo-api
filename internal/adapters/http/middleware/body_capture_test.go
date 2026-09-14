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
		"contacts":[{"EMAIL":"contact@example.invalid"},{"auth.id":"auth0|contact-redact-me"}]
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
	contacts, ok := redacted["contacts"].([]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", contacts[0].(map[string]any)["EMAIL"])
	require.Equal(t, "[REDACTED]", contacts[1].(map[string]any)["auth.id"])
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
