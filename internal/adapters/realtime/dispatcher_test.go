package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDispatcherPublishesAndDeliversLocally(t *testing.T) {
	hub := NewHub()
	connection := &Connection{
		hub:       hub,
		send:      make(chan []byte, 1),
		authID:    "auth0|consumer",
		role:      consumer.Role,
		profileID: 10,
	}
	hub.addConnection(connection)
	eventBus := new(eventBusMock)
	eventBus.On("Publish", mock.Anything, mock.MatchedBy(func(event EventEnvelope) bool {
		return event.ID != "" && event.TargetAuthID == connection.authID
	})).Return(nil).Once()
	dispatcher := NewDispatcher(hub, eventBus)
	payload := []byte(`{"type":"notification.created"}`)

	err := dispatcher.Publish(context.Background(), connection.authID, connection.role, connection.profileID, payload)

	require.NoError(t, err)
	require.Equal(t, payload, <-connection.send)
	eventBus.AssertExpectations(t)
}

func TestDispatcherDoesNotDeliverLocallyWhenDistributionFails(t *testing.T) {
	hub := NewHub()
	connection := &Connection{
		hub:       hub,
		send:      make(chan []byte, 1),
		authID:    "auth0|consumer",
		role:      consumer.Role,
		profileID: 10,
	}
	hub.addConnection(connection)
	eventBus := new(eventBusMock)
	eventBus.On("Publish", mock.Anything, mock.Anything).Return(errors.New("postgres unavailable")).Once()
	dispatcher := NewDispatcher(hub, eventBus)

	err := dispatcher.Publish(
		context.Background(),
		connection.authID,
		connection.role,
		connection.profileID,
		[]byte(`{"type":"notification.created"}`),
	)

	require.ErrorContains(t, err, "postgres unavailable")
	select {
	case <-connection.send:
		t.Fatal("delivered an event that was not distributed")
	default:
	}
	eventBus.AssertExpectations(t)
}

func TestDispatcherDeduplicatesEventsReceivedFromTheBus(t *testing.T) {
	hub := NewHub()
	dispatcher := NewDispatcher(hub, nil)
	connection := &Connection{
		hub:       hub,
		send:      make(chan []byte, 2),
		authID:    "auth0|consumer",
		role:      consumer.Role,
		profileID: 10,
	}
	hub.addConnection(connection)
	event := EventEnvelope{
		ID:              "event-1",
		TargetAuthID:    connection.authID,
		TargetRole:      connection.role,
		TargetProfileID: connection.profileID,
		Payload:         []byte(`{"type":"conversation.message.created"}`),
	}

	dispatcher.receive(event)
	dispatcher.receive(event)

	require.Equal(t, []byte(event.Payload), <-connection.send)
	select {
	case <-connection.send:
		t.Fatal("received duplicate event")
	case <-time.After(25 * time.Millisecond):
	}
}
