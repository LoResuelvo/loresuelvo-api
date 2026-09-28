// Package signedcursor supplies bounded, strict JSON cursor integrity at the HTTP boundary.
package signedcursor

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const MaxLength = 4096

var ErrInvalid = errors.New("invalid signed cursor")

type Codec struct {
	key     []byte
	purpose string
}

// An empty purpose preserves the existing audit-log cursor format. New cursor
// consumers must select a distinct nonempty purpose to prevent cross-use.
func New(key []byte, purpose string) (*Codec, error) {
	if len(key) < 32 {
		return nil, errors.New("cursor signing key must be at least 32 bytes")
	}
	return &Codec{key: bytes.Clone(key), purpose: purpose}, nil
}
func (codec *Codec) signature(data []byte) []byte {
	mac := hmac.New(sha256.New, codec.key)
	if codec.purpose != "" {
		mac.Write([]byte(codec.purpose))
		mac.Write([]byte{0})
	}
	mac.Write(data)
	return mac.Sum(nil)
}
func (codec *Codec) Encode(payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(codec.signature(data))
	if len(token) > MaxLength {
		return "", ErrInvalid
	}
	return token, nil
}
func (codec *Codec) Decode(token string, payload any) error {
	if len(token) == 0 || len(token) > MaxLength {
		return ErrInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ErrInvalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(parts[0])
	if err != nil || base64.RawURLEncoding.EncodeToString(data) != parts[0] {
		return ErrInvalid
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size || base64.RawURLEncoding.EncodeToString(signature) != parts[1] || !hmac.Equal(codec.signature(data), signature) {
		return ErrInvalid
	}
	if err := rejectDuplicateJSONKeys(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		return ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ErrInvalid
	}
	return nil
}

// encoding/json otherwise silently accepts duplicate object keys; cursor payloads
// require one unambiguous value for every signed field, including nested positions.
func rejectDuplicateJSONKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return ErrInvalid
			}
			if _, exists := keys[key]; exists {
				return ErrInvalid
			}
			keys[key] = struct{}{}
			if err := rejectDuplicateJSONKeys(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := rejectDuplicateJSONKeys(decoder); err != nil {
				return err
			}
		}
	default:
		return ErrInvalid
	}
	_, err = decoder.Token()
	return err
}
