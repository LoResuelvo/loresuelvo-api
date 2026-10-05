package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAMQPEventBusRejectsUnsafeConfiguration(t *testing.T) {
	for _, address := range []string{"", "https://broker", "amqp://user:secret@broker.example/vhost", "amqps://user:secret@host:bad/vhost"} {
		_, err := NewAMQPEventBus(address, "test.realtime")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret")
	}
	_, err := NewAMQPEventBus("amqps://user:secret@broker.example/vhost", "amq.fanout")
	require.Error(t, err)
}

func TestAMQPEventBusPublishHonorsCancellationWithoutAListener(t *testing.T) {
	bus, err := NewAMQPEventBus("amqp://localhost/", "test.realtime")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, bus.Publish(ctx, amqpTestEvent()), context.Canceled)
}

func TestAMQPEventBusBroadcastsAcrossInstancesWithoutDuplicates(t *testing.T) {
	address := amqpTestURL(t)
	exchange := "test.realtime." + uuid.NewString()
	cleanupAMQPTestExchange(t, address, exchange)
	busA, err := NewAMQPEventBus(address, exchange)
	require.NoError(t, err)
	busB, err := NewAMQPEventBus(address, exchange)
	require.NoError(t, err)
	hubA, hubB := NewHub(), NewHub()
	dispatcherA, dispatcherB := NewDispatcher(hubA, busA), NewDispatcher(hubB, busB)
	startAMQPTestListener(t, busA, dispatcherA.receive)
	startAMQPTestListener(t, busB, dispatcherB.receive)
	connectionA := &Connection{send: make(chan []byte, 3), authID: "test-user", role: "consumer", profileID: 10}
	connectionB := &Connection{send: make(chan []byte, 3), authID: "test-user", role: "consumer", profileID: 10}
	unrelated := &Connection{send: make(chan []byte, 3), authID: "other-user", role: "consumer", profileID: 10}
	hubA.addConnection(connectionA)
	hubB.addConnection(connectionB)
	hubB.addConnection(unrelated)
	t.Cleanup(hubA.Close)
	t.Cleanup(hubB.Close)
	payload := []byte(fmt.Sprintf(`{"type":"conversation.message.created","content":%q}`, strings.Repeat("message ", 1500)))
	require.Greater(t, len(payload), 8000)

	require.NoError(t, dispatcherA.Publish(context.Background(), "test-user", "consumer", 10, payload))
	for _, connection := range []*Connection{connectionA, connectionB} {
		select {
		case received := <-connection.send:
			require.Equal(t, payload, received)
		case <-time.After(3 * time.Second):
			t.Fatal("instance did not receive broadcast")
		}
	}
	select {
	case <-connectionA.send:
		t.Fatal("local broker echo duplicated the event")
	case <-connectionB.send:
		t.Fatal("remote instance received a duplicate")
	case <-unrelated.send:
		t.Fatal("event leaked to another user")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAMQPEventBusReconnectsAndRecreatesSubscription(t *testing.T) {
	brokerURL := amqpTestURL(t)
	address, err := url.Parse(brokerURL)
	require.NoError(t, err)
	if address.Scheme != "amqp" {
		t.Skip("connection fault injection uses the local plaintext test broker")
	}
	proxyAddress, connections := newAMQPTestProxy(t, address.Host)
	address.Host = proxyAddress
	exchange := "test.realtime." + uuid.NewString()
	cleanupAMQPTestExchange(t, brokerURL, exchange)
	bus, err := NewAMQPEventBus(address.String(), exchange)
	require.NoError(t, err)
	received := make(chan EventEnvelope, 8)
	startAMQPTestListener(t, bus, func(event EventEnvelope) { received <- event })
	require.NoError(t, (<-connections).Close())
	select {
	case <-connections:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not reconnect")
	}
	event := amqpTestEvent()
	require.NoError(t, bus.Publish(context.Background(), event))
	deadline := time.After(3 * time.Second)
	for {
		select {
		case actual := <-received:
			if actual.ID == event.ID {
				require.Equal(t, event, actual)
				return
			}
		case <-deadline:
			t.Fatal("subscription was not restored after reconnect")
		}
	}
}

func TestAMQPEventBusShutdownInterruptsHandshake(t *testing.T) {
	address, connections := newAMQPTestProxy(t, "")
	bus, err := NewAMQPEventBus("amqp://"+address, "test.realtime")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- bus.Listen(ctx, func(EventEnvelope) {}) }()
	<-connections
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on AMQP handshake")
	}
}

func amqpTestURL(t *testing.T) string {
	t.Helper()
	address := os.Getenv("TEST_AMQP_URL")
	if address == "" {
		t.Skip("TEST_AMQP_URL is required for broker integration tests")
	}
	return address
}

func amqpTestEvent() EventEnvelope {
	return EventEnvelope{ID: uuid.NewString(), TargetAuthID: "test-user", TargetRole: "consumer", TargetProfileID: 10, Payload: []byte(`{"type":"notification.created"}`)}
}

func startAMQPTestListener(t *testing.T, bus *AMQPEventBus, handler func(EventEnvelope)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- bus.Listen(ctx, handler) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.True(t, errors.Is(err, context.Canceled), "listener stopped: %v", err)
		case <-time.After(2 * time.Second):
			t.Error("AMQP listener leaked after shutdown")
		}
	})
	// A confirmed publish proves the listener has finished declaring and binding.
	probe := amqpTestEvent()
	probe.TargetAuthID = "readiness-probe"
	require.NoError(t, bus.Publish(ctx, probe))
}
