package realtime

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostgresEventBusDeliversToAHubOnAnotherDatabaseConnection(t *testing.T) {
	databaseA := openRealtimeTestDatabase(t)
	databaseB := openRealtimeTestDatabase(t)
	busA := NewPostgresEventBus(databaseA)
	busB := NewPostgresEventBus(databaseB)
	hubB := NewHub()
	dispatcherB := NewDispatcher(hubB, busB)
	connection := &Connection{
		hub:       hubB,
		send:      make(chan []byte, 1),
		authID:    "realtime-integration-consumer",
		role:      consumer.Role,
		profileID: 10,
	}
	hubB.addConnection(connection)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = dispatcherB.Run(ctx) }()

	payload := []byte(fmt.Sprintf(
		`{"type":"conversation.message.created","content":%q}`,
		strings.Repeat("large distributed message ", 500),
	))
	require.Greater(t, len(payload), 8000)
	event := EventEnvelope{
		ID:              uuid.NewString(),
		TargetAuthID:    connection.authID,
		TargetRole:      connection.role,
		TargetProfileID: connection.profileID,
		Payload:         payload,
	}

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		require.NoError(t, busA.Publish(ctx, event))
		select {
		case received := <-connection.send:
			require.Equal(t, payload, received)
			return
		case <-time.After(50 * time.Millisecond):
			select {
			case <-deadline.C:
				t.Fatal("timed out waiting for event on the second hub")
			default:
			}
		}
	}
}
