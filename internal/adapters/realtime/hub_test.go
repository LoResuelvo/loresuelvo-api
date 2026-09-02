package realtime

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/require"
)

func TestHubDispatchesToAllConnectionsForTheSameParticipant(t *testing.T) {
	hub := NewHub()
	first := &Connection{
		hub:       hub,
		send:      make(chan []byte, 1),
		authID:    "auth0|consumer",
		role:      consumer.Role,
		profileID: 10,
	}
	second := &Connection{
		hub:       hub,
		send:      make(chan []byte, 1),
		authID:    "auth0|consumer",
		role:      consumer.Role,
		profileID: 10,
	}
	hub.addConnection(first)
	hub.addConnection(second)

	payload := []byte(`{"type":"notification.created"}`)
	hub.Deliver(EventEnvelope{
		ID:              "event-1",
		TargetAuthID:    first.authID,
		TargetRole:      first.role,
		TargetProfileID: first.profileID,
		Payload:         payload,
	})

	select {
	case received := <-first.send:
		require.Equal(t, payload, received)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first connection")
	}
	select {
	case received := <-second.send:
		require.Equal(t, payload, received)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second connection")
	}
}

func TestHubRemovingOneConnectionKeepsOtherConnections(t *testing.T) {
	hub := NewHub()
	first := &Connection{hub: hub, send: make(chan []byte, 1), authID: "auth0|consumer", role: consumer.Role, profileID: 10}
	second := &Connection{hub: hub, send: make(chan []byte, 1), authID: "auth0|consumer", role: consumer.Role, profileID: 10}
	hub.addConnection(first)
	hub.addConnection(second)
	hub.removeConnection(first)

	payload := []byte(`{"type":"notification.created"}`)
	hub.Deliver(EventEnvelope{
		ID:              "event-2",
		TargetAuthID:    second.authID,
		TargetRole:      second.role,
		TargetProfileID: second.profileID,
		Payload:         payload,
	})
	require.Equal(t, payload, <-second.send)
	_, open := <-first.send
	require.False(t, open)
}
