package audit_log_handler

import (
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
)

const cursorVersion = 1

var errInvalidCursor = errors.New("invalid audit cursor")

type cursorCodec struct {
	codec *signedcursor.Codec
}

type cursorPayload struct {
	Version   int               `json:"v"`
	Watermark int64             `json:"watermark"`
	Before    audit.LogPosition `json:"before"`
	Filter    audit.LogFilter   `json:"filter"`
	Limit     int               `json:"limit"`
}

func newCursorCodec(key []byte) (*cursorCodec, error) {
	codec, err := signedcursor.New(key, "")
	if err != nil {
		return nil, err
	}
	return &cursorCodec{codec: codec}, nil
}
func (codec *cursorCodec) encode(payload cursorPayload) (string, error) {
	payload.Version = cursorVersion
	return codec.codec.Encode(payload)
}
func (codec *cursorCodec) decode(token string) (cursorPayload, error) {
	var payload cursorPayload
	if err := codec.codec.Decode(token, &payload); err != nil {
		return cursorPayload{}, errInvalidCursor
	}
	if payload.Version != cursorVersion || payload.Watermark < 0 || payload.Limit < 1 || payload.Limit > 100 || payload.Filter.Validate() != nil || payload.Before.OccurredOn.IsZero() || payload.Before.ID == uuid.Nil {
		return cursorPayload{}, errInvalidCursor
	}
	return payload, nil
}
