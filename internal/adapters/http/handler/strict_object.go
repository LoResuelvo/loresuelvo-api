package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"unicode/utf8"
)

var ErrInvalidJSONObject = errors.New("invalid JSON object")

// StrictObject preserves the distinction between absent and null fields and rejects ambiguous duplicate members.
func StrictObject(data []byte, allowed ...string) (map[string]json.RawMessage, error) {
	if !utf8.Valid(data) {
		return nil, ErrInvalidJSONObject
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrInvalidJSONObject
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, ErrInvalidJSONObject
		}
		key, ok := token.(string)
		if !ok || !slices.Contains(allowed, key) {
			return nil, ErrInvalidJSONObject
		}
		if _, exists := fields[key]; exists {
			return nil, ErrInvalidJSONObject
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, ErrInvalidJSONObject
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, ErrInvalidJSONObject
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, ErrInvalidJSONObject
	}
	return fields, nil
}
