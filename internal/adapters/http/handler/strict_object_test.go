package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrictObjectRejectsAmbiguousDocuments(t *testing.T) {
	for _, body := range []string{`[]`, `null`, `{"name":1,"name":2}`, `{"unknown":1}`, `{} {}`, `{} trailing`, "{\"name\":\"\xff\"}"} {
		_, err := StrictObject([]byte(body), "name")
		require.ErrorIs(t, err, ErrInvalidJSONObject)
	}
}
func TestStrictObjectPreservesNullAndAbsence(t *testing.T) {
	fields, err := StrictObject([]byte(`{"name":null}`), "name", "other")
	require.NoError(t, err)
	require.Equal(t, "null", string(fields["name"]))
	_, exists := fields["other"]
	require.False(t, exists)
}
