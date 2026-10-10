package realtime

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type eventBusMock struct {
	mock.Mock
}

// newAMQPTestProxy allows a real broker connection to be interrupted without
// stopping a shared broker. An empty upstream deliberately stalls the handshake.
func newAMQPTestProxy(t *testing.T, upstream string) (string, <-chan net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	connections := make(chan net.Conn, 8)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			connections <- client
			if upstream == "" {
				t.Cleanup(func() { closeAMQPTransport(client) })
				continue
			}
			server, err := net.DialTimeout("tcp", upstream, time.Second)
			if err != nil {
				closeAMQPTransport(client)
				return
			}
			copyStream := func(dst, src net.Conn) {
				defer closeAMQPTransport(dst)
				defer closeAMQPTransport(src)
				if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, net.ErrClosed) {
					t.Logf("test proxy interrupted: %v", err)
				}
			}
			go copyStream(server, client)
			go copyStream(client, server)
		}
	}()
	return listener.Addr().String(), connections
}

func cleanupAMQPTestExchange(t *testing.T, address, exchange string) {
	t.Helper()
	t.Cleanup(func() {
		connection, err := amqp.DialConfig(address, amqp.Config{Dial: amqp.DefaultDial(time.Second)})
		require.NoError(t, err)
		defer func() { require.NoError(t, connection.CloseDeadline(time.Now().Add(time.Second))) }()
		channel, err := connection.Channel()
		require.NoError(t, err)
		require.NoError(t, channel.ExchangeDelete(exchange, false, false))
	})
}

func (bus *eventBusMock) Publish(ctx context.Context, event EventEnvelope) error {
	return bus.Called(ctx, event).Error(0)
}

func (bus *eventBusMock) Listen(ctx context.Context, handler func(EventEnvelope)) error {
	return bus.Called(ctx, handler).Error(0)
}

type notificationRecipientFinderStub struct {
	authID string
	role   string
	err    error
}

func (finder notificationRecipientFinderStub) FindByID(_ context.Context, id int) (user.User, error) {
	if finder.err != nil {
		return nil, finder.err
	}
	base := user.RehydrateBaseUser(id, finder.authID, "", "", "", finder.role, nil)
	if finder.role == provider.Role {
		return &provider.Provider{BaseUser: base}, nil
	}
	return &consumer.Consumer{BaseUser: base}, nil
}

type notificationJobRequestFinderMock struct{ mock.Mock }

func (m *notificationJobRequestFinderMock) FindByID(id int) (*jobrequest.JobRequest, error) {
	args := m.Called(id)
	request, _ := args.Get(0).(*jobrequest.JobRequest)
	return request, args.Error(1)
}

type notificationImageResolverMock struct{ mock.Mock }

func (m *notificationImageResolverMock) ResolveJobRequestImages(ctx context.Context, images []filedomain.Image) ([]filedomain.Image, error) {
	args := m.Called(ctx, images)
	found, _ := args.Get(0).([]filedomain.Image)
	return found, args.Error(1)
}

type notificationUserFinderMock struct{ mock.Mock }

func (m *notificationUserFinderMock) FindByID(ctx context.Context, id int) (user.User, error) {
	args := m.Called(ctx, id)
	found, _ := args.Get(0).(user.User)
	return found, args.Error(1)
}

type notificationDispatcherMock struct{ mock.Mock }

func (m *notificationDispatcherMock) Publish(ctx context.Context, authID, role string, id int, event []byte) error {
	return m.Called(ctx, authID, role, id, event).Error(0)
}
