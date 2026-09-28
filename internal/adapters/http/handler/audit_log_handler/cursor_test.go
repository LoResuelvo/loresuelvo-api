package audit_log_handler

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCursorCodecRejectsShortKey(t *testing.T) {
	_, err := newCursorCodec([]byte("short"))
	require.Error(t, err)
}

func TestCursorCodecRoundTripAndTamperRejection(t *testing.T) {
	codec, err := newCursorCodec([]byte("test-audit-cursor-signing-key-2026-keep-private"))
	require.NoError(t, err)
	operatorID := 42
	action := audit.ActionCreate
	from := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	payload := cursorPayload{
		Watermark: 71,
		Before:    audit.LogPosition{OccurredOn: from.Add(time.Hour), ID: uuid.New()},
		Filter:    audit.LogFilter{OperatorID: &operatorID, Action: &action, OccurredFrom: &from},
		Limit:     20,
	}
	token, err := codec.encode(payload)
	require.NoError(t, err)
	got, err := codec.decode(token)
	require.NoError(t, err)
	require.Equal(t, cursorVersion, got.Version)
	require.Equal(t, payload.Watermark, got.Watermark)
	require.Equal(t, payload.Before, got.Before)
	require.Equal(t, payload.Filter, got.Filter)
	require.Equal(t, payload.Limit, got.Limit)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 2)
	bytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var changed map[string]any
	require.NoError(t, json.Unmarshal(bytes, &changed))
	changed["limit"] = float64(100)
	modified, err := json.Marshal(changed)
	require.NoError(t, err)
	_, err = codec.decode(base64.RawURLEncoding.EncodeToString(modified) + "." + parts[1])
	require.ErrorIs(t, err, errInvalidCursor)

	for _, invalid := range []string{"", "not-a-cursor", token + ".extra", strings.Repeat("a", signedcursor.MaxLength+1)} {
		_, err = codec.decode(invalid)
		require.ErrorIs(t, err, errInvalidCursor)
	}
}

func TestCursorCodecRejectsSignedInvalidPayloads(t *testing.T) {
	codec, err := newCursorCodec([]byte("test-audit-cursor-signing-key-2026-keep-private"))
	require.NoError(t, err)
	valid := cursorPayload{
		Watermark: 71,
		Before:    audit.LogPosition{OccurredOn: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), ID: uuid.New()},
		Limit:     20,
	}
	for name, change := range map[string]func(*cursorPayload){
		"negative watermark": func(p *cursorPayload) { p.Watermark = -1 },
		"invalid limit":      func(p *cursorPayload) { p.Limit = 101 },
		"missing position":   func(p *cursorPayload) { p.Before = audit.LogPosition{} },
	} {
		t.Run(name, func(t *testing.T) {
			payload := valid
			change(&payload)
			token, err := codec.encode(payload)
			require.NoError(t, err)
			_, err = codec.decode(token)
			require.ErrorIs(t, err, errInvalidCursor)
		})
	}
}

// The fixed token was generated with the original audit codec's raw JSON HMAC
// (no purpose prefix). It protects compatibility independently of round-trips.
func TestCursorCodecPreservesLegacyTokenBytes(t *testing.T) {
	codec, err := newCursorCodec([]byte("test-audit-cursor-signing-key-2026-keep-private"))
	require.NoError(t, err)
	const legacyToken = "eyJ2IjoxLCJ3YXRlcm1hcmsiOjcxLCJiZWZvcmUiOnsiT2NjdXJyZWRPbiI6IjIwMjYtMDktMjBUMDE6MDA6MDBaIiwiSUQiOiIwMDAwMDAwMC0wMDAwLTQwMDAtODAwMC0wMDAwMDAwMDAwMDEifSwiZmlsdGVyIjp7Ik9wZXJhdG9ySUQiOm51bGwsIkFjdGlvbiI6bnVsbCwiUmVzb3VyY2VUeXBlIjpudWxsLCJSZXNvdXJjZUlEIjpudWxsLCJSZXN1bHQiOm51bGwsIk9jY3VycmVkRnJvbSI6bnVsbCwiT2NjdXJyZWRUbyI6bnVsbH0sImxpbWl0IjoyMH0.Jqm-sSyWn0tn5WseAm630sUG4F-nBWoFlgl9bBLo7wA"
	expected := cursorPayload{
		Version: 1, Watermark: 71,
		Before: audit.LogPosition{OccurredOn: time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC), ID: uuid.MustParse("00000000-0000-4000-8000-000000000001")},
		Limit:  20,
	}
	decoded, err := codec.decode(legacyToken)
	require.NoError(t, err)
	require.Equal(t, expected, decoded)
	encoded, err := codec.encode(expected)
	require.NoError(t, err)
	require.Equal(t, legacyToken, encoded)
}
