package operation_chat_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"net/url"
	"strconv"
	"time"
)

const chatCursorVersion = 1
const chatCursorPurpose = "operation_chat:v1"

type cursorPayload struct {
	Version        int            `json:"v"`
	OperationID    string         `json:"operation_id"`
	ConversationID int            `json:"conversation_id"`
	Limit          int            `json:"limit"`
	After          cursorPosition `json:"after"`
}

// cursorPosition belongs to the HTTP wire format, not the domain ordering key.
type cursorPosition struct {
	CreatedOn time.Time `json:"created_on"`
	ID        int       `json:"id"`
}

func parseQuery(rawQuery, rawOperationID string, codec *signedcursor.Codec) (operation.ChatQuery, error) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return operation.ChatQuery{}, operation.ErrInvalidChatQuery
	}
	query := operation.ChatQuery{}
	for key, value := range values {
		if (key != "limit" && key != "cursor") || len(value) != 1 || value[0] == "" {
			return query, operation.ErrInvalidChatQuery
		}
	}
	if rawLimit := values.Get("limit"); rawLimit != "" {
		for _, digit := range rawLimit {
			if digit < '0' || digit > '9' {
				return query, operation.ErrInvalidChatQuery
			}
		}
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > operation.MaxChatPageSize {
			return query, operation.ErrInvalidChatQuery
		}
		query.Limit = limit
	}
	if token := values.Get("cursor"); token != "" {
		var payload cursorPayload
		if err := codec.Decode(token, &payload); err != nil || payload.Version != chatCursorVersion || payload.OperationID != rawOperationID || payload.Limit < 1 || payload.Limit > operation.MaxChatPageSize {
			return query, operation.ErrInvalidChatQuery
		}
		if _, err := operation.ParseOperationID(payload.OperationID); err != nil {
			return query, operation.ErrInvalidChatQuery
		}
		if query.Limit != 0 && query.Limit != payload.Limit {
			return query, operation.ErrInvalidChatQuery
		}
		query = operation.ChatQuery{Limit: payload.Limit, ConversationID: payload.ConversationID, After: &conversation.MessagePosition{CreatedOn: payload.After.CreatedOn, ID: payload.After.ID}}
	}
	if err := query.Validate(); err != nil {
		return query, err
	}
	return query, nil
}
