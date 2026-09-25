package operation_inbox_handler

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

func TestCursorRoundTripsPositionFilterAndLimit(t *testing.T) {
	consumerID := 7
	from := time.Date(2026, 9, 20, 3, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	stage := readmodel.StageProposalPending
	filter := operation.InboxFilter{
		ConsumerID: &consumerID, StartedFrom: &from, Stage: &stage,
		ScheduledDay: &operation.CalendarDay{Year: 2026, Month: time.September, Day: 25},
	}
	position := operation.InboxPosition{StartedOn: from, ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 34}}

	token, err := encodeCursor(position, filter, 2)
	require.NoError(t, err)
	payload, err := decodeCursor(token)

	require.NoError(t, err)
	require.Equal(t, 2, payload.Limit)
	require.True(t, payload.position().StartedOn.Equal(from))
	require.Equal(t, position.ID, payload.position().ID)
	require.True(t, payload.Filter.accepts(filter))
	require.True(t, payload.Filter.accepts(operation.InboxFilter{}))
}

func TestCursorRejectsMalformedTokens(t *testing.T) {
	encode := func(raw string) string { return base64.RawURLEncoding.EncodeToString([]byte(raw)) }
	valid := `{"v":1,"started_on":"2026-09-25T15:00:00Z","kind":"jr","resource_id":1,"filter":{},"limit":2}`
	for name, token := range map[string]string{
		"not base64":      "no-es-un-cursor",
		"unknown version": encode(strings.Replace(valid, `"v":1`, `"v":2`, 1)),
		"unknown field":   encode(strings.Replace(valid, `"limit":2`, `"limit":2,"extra":true`, 1)),
		"trailing data":   encode(valid + `{}`),
		"limit too high":  encode(strings.Replace(valid, `"limit":2`, `"limit":101`, 1)),
		"too long":        strings.Repeat("a", maxCursorLength+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeCursor(token)
			require.ErrorIs(t, err, errInvalidCursor)
		})
	}
}

func TestCursorRejectsFiltersThatDifferFromItsOwn(t *testing.T) {
	original, other := 7, 8
	stage := readmodel.StageProposalPending
	token, err := encodeCursor(
		operation.InboxPosition{StartedOn: time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC), ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 1}},
		operation.InboxFilter{ConsumerID: &original}, 2,
	)
	require.NoError(t, err)
	payload, err := decodeCursor(token)
	require.NoError(t, err)

	require.False(t, payload.Filter.accepts(operation.InboxFilter{ConsumerID: &other}))
	require.False(t, payload.Filter.accepts(operation.InboxFilter{Stage: &stage}))
}
