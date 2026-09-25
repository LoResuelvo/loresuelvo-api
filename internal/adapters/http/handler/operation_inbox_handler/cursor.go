package operation_inbox_handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

const (
	cursorVersion   = 1
	maxCursorLength = 2048
)

var errInvalidCursor = errors.New("invalid operations inbox cursor")

type cursorPayload struct {
	Version    int            `json:"v"`
	StartedOn  time.Time      `json:"started_on"`
	Kind       readmodel.Kind `json:"kind"`
	ResourceID int            `json:"resource_id"`
	Filter     cursorFilter   `json:"filter"`
	Limit      int            `json:"limit"`
}

type cursorFilter struct {
	ConsumerID   *int                   `json:"consumer_id,omitempty"`
	ProviderID   *int                   `json:"provider_id,omitempty"`
	CategoryID   *int                   `json:"category_id,omitempty"`
	StartedFrom  *time.Time             `json:"started_from,omitempty"`
	StartedTo    *time.Time             `json:"started_to,omitempty"`
	Stage        *readmodel.Stage       `json:"stage,omitempty"`
	Alert        *readmodel.Alert       `json:"alert,omitempty"`
	ScheduledDay *operation.CalendarDay `json:"scheduled_day,omitempty"`
}

func encodeCursor(position operation.InboxPosition, filter operation.InboxFilter, limit int) (string, error) {
	data, err := json.Marshal(cursorPayload{
		Version: cursorVersion, StartedOn: position.StartedOn.UTC(), Kind: position.ID.Kind,
		ResourceID: position.ID.ResourceID, Filter: cursorFilter(filter), Limit: limit,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeCursor(token string) (cursorPayload, error) {
	if len(token) > maxCursorLength {
		return cursorPayload{}, errInvalidCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return cursorPayload{}, errInvalidCursor
	}
	var payload cursorPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return cursorPayload{}, errInvalidCursor
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return cursorPayload{}, errInvalidCursor
	}
	if payload.Version != cursorVersion || payload.Limit < 1 || payload.Limit > operation.MaxInboxLimit {
		return cursorPayload{}, errInvalidCursor
	}
	return payload, nil
}

func (payload cursorPayload) position() operation.InboxPosition {
	return operation.InboxPosition{
		StartedOn: payload.StartedOn.UTC(),
		ID:        readmodel.ID{Kind: payload.Kind, ResourceID: payload.ResourceID},
	}
}

func sameIfSet[T comparable](explicit, original *T) bool {
	return explicit == nil || (original != nil && *explicit == *original)
}

func sameInstantIfSet(explicit, original *time.Time) bool {
	return explicit == nil || (original != nil && explicit.Equal(*original))
}

func (original cursorFilter) accepts(explicit operation.InboxFilter) bool {
	return sameIfSet(explicit.ConsumerID, original.ConsumerID) &&
		sameIfSet(explicit.ProviderID, original.ProviderID) &&
		sameIfSet(explicit.CategoryID, original.CategoryID) &&
		sameInstantIfSet(explicit.StartedFrom, original.StartedFrom) &&
		sameInstantIfSet(explicit.StartedTo, original.StartedTo) &&
		sameIfSet(explicit.Stage, original.Stage) &&
		sameIfSet(explicit.Alert, original.Alert) &&
		sameIfSet(explicit.ScheduledDay, original.ScheduledDay)
}
