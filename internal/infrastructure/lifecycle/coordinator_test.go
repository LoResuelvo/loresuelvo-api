package lifecycle

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type serverStub struct {
	listenStarted chan struct{}
	listenDone    chan struct{}
	shutdown      chan struct{}
	closeCalled   chan struct{}
	shutdownErr   error
	closeErr      error
	listenOnce    sync.Once
}

func newServerStub() *serverStub {
	return &serverStub{
		listenStarted: make(chan struct{}),
		listenDone:    make(chan struct{}),
		shutdown:      make(chan struct{}),
		closeCalled:   make(chan struct{}),
	}
}

func (server *serverStub) ListenAndServe() error {
	close(server.listenStarted)
	<-server.listenDone
	return http.ErrServerClosed
}

func (server *serverStub) Shutdown(context.Context) error {
	close(server.shutdown)
	server.listenOnce.Do(func() { close(server.listenDone) })
	return server.shutdownErr
}

func (server *serverStub) Close() error {
	select {
	case <-server.closeCalled:
	default:
		close(server.closeCalled)
	}
	server.listenOnce.Do(func() { close(server.listenDone) })
	return server.closeErr
}

type readinessStub struct {
	marked chan struct{}
}

func (readiness *readinessStub) MarkDraining() {
	close(readiness.marked)
}

type hubStub struct {
	closed chan struct{}
}

func (hub *hubStub) Close() {
	close(hub.closed)
}

type closerStub struct {
	closed chan struct{}
}

func (closer *closerStub) Close() error {
	close(closer.closed)
	return nil
}

func TestShutdownTimeoutFromEnv(t *testing.T) {
	t.Run("uses default", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "")

		timeout, err := ShutdownTimeoutFromEnv()

		require.NoError(t, err)
		require.Equal(t, DefaultShutdownTimeout, timeout)
	})

	t.Run("parses positive duration", func(t *testing.T) {
		t.Setenv("SHUTDOWN_TIMEOUT", "45s")

		timeout, err := ShutdownTimeoutFromEnv()

		require.NoError(t, err)
		require.Equal(t, 45*time.Second, timeout)
	})

	for _, value := range []string{"invalid", "45", "0", "-1s"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", value)

			_, err := ShutdownTimeoutFromEnv()

			require.Error(t, err)
		})
	}
}

func TestCoordinatorDrainsAndClosesResourcesAfterWorkersStop(t *testing.T) {
	server := newServerStub()
	readiness := &readinessStub{marked: make(chan struct{})}
	hub := &hubStub{closed: make(chan struct{})}
	closer := &closerStub{closed: make(chan struct{})}
	workerStarted := make(chan struct{})
	workerStopped := make(chan struct{})
	var orderMu sync.Mutex
	order := make([]string, 0, 5)
	appendOrder := func(value string) {
		orderMu.Lock()
		defer orderMu.Unlock()
		order = append(order, value)
	}

	coordinator, err := NewCoordinator(Config{
		Server:          server,
		Readiness:       readiness,
		Hub:             hub,
		ShutdownTimeout: time.Second,
		Workers: []Worker{func(ctx context.Context) error {
			close(workerStarted)
			<-ctx.Done()
			appendOrder("worker")
			close(workerStopped)
			return nil
		}},
		Closers: []io.Closer{ioCloserFunc(func() error {
			appendOrder("closer")
			close(closer.closed)
			return nil
		})},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- coordinator.Run(ctx) }()

	<-server.listenStarted
	<-workerStarted
	cancel()

	require.NoError(t, <-runDone)
	<-readiness.marked
	<-hub.closed
	<-server.shutdown
	<-workerStopped
	<-closer.closed
	orderMu.Lock()
	require.Equal(t, []string{"worker", "closer"}, order)
	orderMu.Unlock()
}

func TestCoordinatorForcesServerCloseWhenGracefulShutdownFails(t *testing.T) {
	server := newServerStub()
	server.shutdownErr = context.DeadlineExceeded
	readiness := &readinessStub{marked: make(chan struct{})}

	coordinator, err := NewCoordinator(Config{
		Server:          server,
		Readiness:       readiness,
		ShutdownTimeout: time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- coordinator.Run(ctx) }()
	<-server.listenStarted
	cancel()

	returnedErr := <-runDone
	require.ErrorIs(t, returnedErr, context.DeadlineExceeded)
	<-server.closeCalled
}

type ioCloserFunc func() error

func (function ioCloserFunc) Close() error {
	return function()
}
