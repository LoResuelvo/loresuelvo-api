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

func TestHubHasConnectionsForAuthIDTracksRegistrationLifecycle(t *testing.T) {
	hub := NewHub()
	require.False(t, hub.HasConnectionsForAuthID("auth0|consumer"))
	first := &Connection{hub: hub, send: make(chan []byte, 1), authID: "auth0|consumer", role: consumer.Role, profileID: 10}
	second := &Connection{hub: hub, send: make(chan []byte, 1), authID: first.authID, role: first.role, profileID: first.profileID}
	hub.addConnection(first)
	hub.addConnection(second)
	require.True(t, hub.HasConnectionsForAuthID(first.authID))
	require.False(t, hub.HasConnectionsForAuthID("auth0|other"))
	hub.removeConnection(first)
	require.True(t, hub.HasConnectionsForAuthID(second.authID))
	hub.removeConnection(second)
	require.False(t, hub.HasConnectionsForAuthID(second.authID))
}

func TestHubHasConnectionsForAuthIDIsAbsentAfterClose(t *testing.T) {
	hub := NewHub()
	connection := &Connection{hub: hub, send: make(chan []byte, 1), authID: "auth0|consumer", role: consumer.Role, profileID: 10}
	hub.addConnection(connection)
	require.True(t, hub.HasConnectionsForAuthID(connection.authID))
	hub.Close()
	require.False(t, hub.HasConnectionsForAuthID(connection.authID))
	var absent *Hub
	require.False(t, absent.HasConnectionsForAuthID(connection.authID))
	require.False(t, hub.HasConnectionsForAuthID(""))
}
