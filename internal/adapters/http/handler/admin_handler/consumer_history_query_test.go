package admin_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/stretchr/testify/require"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConsumerHistoryQueryRejectsAmbiguousUnboundedInput(t *testing.T) {
	codec, err := signedcursor.New([]byte(strings.Repeat("k", 32)), consumerHistoryCursorPurpose)
	require.NoError(t, err)
	for _, raw := range []string{"type=unknown", "status=paid", "type=job_request&status=paid", "limit=0", "limit=101", "limit=-1", "limit=2147483648", "limit=2&limit=2", "from=", "provider_id=+1", "provider_id=2147483648", "to=bad", "from=2026-09-20T00:00:00Z&to=2026-09-20T00:00:00Z", "foo=1", "cursor=bad", "%zz=1"} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseConsumerHistoryQuery(raw, 12, codec)
			require.ErrorIs(t, err, admin.ErrInvalidConsumerHistoryQuery)
		})
	}
	q, err := parseConsumerHistoryQuery("", 12, codec)
	require.NoError(t, err)
	require.Equal(t, 20, q.Limit)
	for _, raw := range []string{"0", "-1", "+1", "1.1", "2147483648", "abc", ""} {
		_, err := parseConsumerHistoryID(raw)
		require.Error(t, err)
	}
}
func TestConsumerHistoryCursorInheritsScopeAndRejectsChanges(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	codec, err := signedcursor.New(key, consumerHistoryCursorPurpose)
	require.NoError(t, err)
	from := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	p := consumerHistoryCursor{Version: 1, ConsumerID: 12, Type: "work_order", Status: "paid", ProviderID: 23, From: &from, Limit: 2, After: readmodel.ConsumerHistoryPosition{OccurredOn: from.Add(time.Hour), Type: "work_order", ID: 99}}
	token, err := codec.Encode(p)
	require.NoError(t, err)
	raw := "cursor=" + url.QueryEscape(token)
	q, err := parseConsumerHistoryQuery(raw, 12, codec)
	require.NoError(t, err)
	require.Equal(t, 2, q.Limit)
	require.Equal(t, 23, q.ProviderID)
	require.Equal(t, &p.After, q.After)
	q, err = parseConsumerHistoryQuery(raw+"&from=2026-09-19T21:00:00-03:00&limit=2&type=work_order", 12, codec)
	require.NoError(t, err)
	require.True(t, q.From.Equal(from))
	for _, extra := range []string{"&type=job_request", "&status=scheduled", "&provider_id=24", "&from=2026-09-21T00:00:00Z", "&to=2026-10-01T00:00:00Z", "&limit=3"} {
		_, err := parseConsumerHistoryQuery(raw+extra, 12, codec)
		require.ErrorIs(t, err, admin.ErrInvalidConsumerHistoryQuery)
	}
	_, err = parseConsumerHistoryQuery(raw, 13, codec)
	require.ErrorIs(t, err, admin.ErrInvalidConsumerHistoryQuery)
	other, err := signedcursor.New(key, "operation_chat:v1")
	require.NoError(t, err)
	foreign, err := other.Encode(p)
	require.NoError(t, err)
	_, err = parseConsumerHistoryQuery("cursor="+foreign, 12, codec)
	require.ErrorIs(t, err, admin.ErrInvalidConsumerHistoryQuery)
	_, err = parseConsumerHistoryQuery("cursor="+token[:len(token)-3]+"aaa", 12, codec)
	require.ErrorIs(t, err, admin.ErrInvalidConsumerHistoryQuery)
}

func TestConsumerHistoryCursorPreservesNanosecondWindow(t *testing.T) {
	codec, err := signedcursor.New([]byte(strings.Repeat("k", 32)), consumerHistoryCursorPurpose)
	require.NoError(t, err)
	raw := "from=2026-09-20T00:00:00.000000001Z&to=2026-09-20T00:00:00.000001501Z&limit=1"
	q, err := parseConsumerHistoryQuery(raw, 12, codec)
	require.NoError(t, err)
	require.Equal(t, 1, q.From.Nanosecond())
	require.Equal(t, 1501, q.To.Nanosecond())
	position := readmodel.ConsumerHistoryPosition{OccurredOn: q.From.Truncate(time.Microsecond).Add(time.Microsecond), Type: "work_order", ID: 99}
	token, err := codec.Encode(consumerHistoryCursor{Version: 1, ConsumerID: 12, From: q.From, To: q.To, Limit: q.Limit, After: position})
	require.NoError(t, err)
	next, err := parseConsumerHistoryQuery("cursor="+token, 12, codec)
	require.NoError(t, err)
	require.Equal(t, q.From, next.From)
	require.Equal(t, q.To, next.To)
	require.Equal(t, &position, next.After)
	_, err = parseConsumerHistoryQuery("cursor="+token+"&from=2026-09-19T21:00:00.000000001-03:00", 12, codec)
	require.NoError(t, err)
}
