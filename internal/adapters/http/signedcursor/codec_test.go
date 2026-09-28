package signedcursor

import (
	"encoding/base64"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestCodecPurposeIsolationIntegrityAndStrictJSON(t *testing.T) {
	key := []byte("test-audit-cursor-signing-key-2026-keep-private")
	codec, err := New(key, "chat")
	require.NoError(t, err)
	token, err := codec.Encode(struct {
		ID int `json:"id"`
	}{ID: 4})
	require.NoError(t, err)
	var payload struct {
		ID int `json:"id"`
	}
	require.NoError(t, codec.Decode(token, &payload))
	require.Equal(t, 4, payload.ID)
	other, err := New(key, "audit")
	require.NoError(t, err)
	require.ErrorIs(t, other.Decode(token, &payload), ErrInvalid)
	for _, invalid := range []string{"", token + ".extra", strings.Repeat("x", MaxLength+1), "x" + token} {
		require.ErrorIs(t, codec.Decode(invalid, &payload), ErrInvalid)
	}
	unknown, err := codec.Encode(map[string]int{"other": 1})
	require.NoError(t, err)
	require.ErrorIs(t, codec.Decode(unknown, &payload), ErrInvalid)
	duplicate, err := codec.Encode(json.RawMessage(`{"id":4,"id":5}`))
	require.NoError(t, err)
	require.ErrorIs(t, codec.Decode(duplicate, &payload), ErrInvalid)
	// Sign deliberately invalid concatenated JSON; Encode correctly refuses it,
	// so construct the authenticated malformed input explicitly for Decode.
	trailingData := []byte(`{"id":4} {}`)
	trailing := base64.RawURLEncoding.EncodeToString(trailingData) + "." + base64.RawURLEncoding.EncodeToString(codec.signature(trailingData))
	require.ErrorIs(t, codec.Decode(trailing, &payload), ErrInvalid)
	_, err = New([]byte("short"), "chat")
	require.Error(t, err)
}
