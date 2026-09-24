package audit_log_handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
)

const cursorVersion = 1
const maxCursorLength = 4096

var errInvalidCursor = errors.New("invalid audit cursor")

type cursorCodec struct {
	key []byte
}

type cursorPayload struct {
	Version   int               `json:"v"`
	Watermark int64             `json:"watermark"`
	Before    audit.LogPosition `json:"before"`
	Filter    audit.LogFilter   `json:"filter"`
	Limit     int               `json:"limit"`
}

func newCursorCodec(key []byte) (*cursorCodec, error) {
	if len(key) < 32 {
		return nil, errors.New("audit cursor signing key must be at least 32 bytes")
	}
	return &cursorCodec{key: bytes.Clone(key)}, nil
}

func (codec *cursorCodec) encode(payload cursorPayload) (string, error) {
	payload.Version = cursorVersion
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, codec.key)
	_, _ = mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (codec *cursorCodec) decode(token string) (cursorPayload, error) {
	if len(token) == 0 || len(token) > maxCursorLength {
		return cursorPayload{}, errInvalidCursor
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return cursorPayload{}, errInvalidCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return cursorPayload{}, errInvalidCursor
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size {
		return cursorPayload{}, errInvalidCursor
	}
	mac := hmac.New(sha256.New, codec.key)
	_, _ = mac.Write(data)
	if !hmac.Equal(mac.Sum(nil), signature) {
		return cursorPayload{}, errInvalidCursor
	}

	var payload cursorPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return cursorPayload{}, errInvalidCursor
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return cursorPayload{}, errInvalidCursor
	}
	if payload.Version != cursorVersion || payload.Watermark < 0 ||
		payload.Limit < 1 || payload.Limit > 100 || payload.Filter.Validate() != nil ||
		payload.Before.OccurredOn.IsZero() || payload.Before.ID == uuid.Nil {
		return cursorPayload{}, errInvalidCursor
	}
	return payload, nil
}
