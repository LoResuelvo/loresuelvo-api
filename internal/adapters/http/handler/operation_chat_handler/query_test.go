package operation_chat_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestParseChatQueryValidatesBoundedSignedContinuation(t *testing.T) {
	key := []byte("test-audit-cursor-signing-key-2026-keep-private")
	codec, err := signedcursor.New(key, chatCursorPurpose)
	require.NoError(t, err)
	payload := cursorPayload{Version: chatCursorVersion, OperationID: "jr-12", ConversationID: 9, Limit: 2, After: cursorPosition{ID: 4, CreatedOn: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)}}
	token, err := codec.Encode(payload)
	require.NoError(t, err)
	for _, raw := range []string{"cursor=" + token, "cursor=" + token + "&limit=2"} {
		query, err := parseQuery(raw, "jr-12", codec)
		require.NoError(t, err)
		require.Equal(t, 2, query.Limit)
		require.Equal(t, 9, query.ConversationID)
		require.Equal(t, conversation.MessagePosition{ID: payload.After.ID, CreatedOn: payload.After.CreatedOn}, *query.After)
	}
	for _, raw := range []string{"limit=0", "limit=101", "limit=-1", "limit=+2", "limit=abc", "limit=2&limit=2", "limit=", "cursor=", "cursor=invalid", "cursor=" + token + "&limit=3", "cursor=" + token + "&cursor=" + token, "conversation_id=9", "limit=2;cursor=foo", "limit=%ZZ"} {
		_, err := parseQuery(raw, "jr-12", codec)
		require.ErrorIs(t, err, operation.ErrInvalidChatQuery, raw)
	}
	_, err = parseQuery("cursor="+token, "sp-12", codec)
	require.ErrorIs(t, err, operation.ErrInvalidChatQuery)
	for _, mutate := range []func(*cursorPayload){func(p *cursorPayload) { p.Version = 2 }, func(p *cursorPayload) { p.ConversationID = 0 }, func(p *cursorPayload) { p.After.ID = 0 }, func(p *cursorPayload) { p.After.CreatedOn = time.Time{} }, func(p *cursorPayload) { p.Limit = 0 }} {
		invalid := payload
		mutate(&invalid)
		bad, err := codec.Encode(invalid)
		require.NoError(t, err)
		_, err = parseQuery("cursor="+bad, "jr-12", codec)
		require.ErrorIs(t, err, operation.ErrInvalidChatQuery)
	}
	otherCodec, err := signedcursor.New(key, "")
	require.NoError(t, err)
	otherToken, err := otherCodec.Encode(payload)
	require.NoError(t, err)
	_, err = parseQuery("cursor="+otherToken, "jr-12", codec)
	require.ErrorIs(t, err, operation.ErrInvalidChatQuery)
}
