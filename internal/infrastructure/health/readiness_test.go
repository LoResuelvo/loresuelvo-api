package health

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type pingerStub struct {
	err   error
	calls atomic.Int32
}

func (stub *pingerStub) PingContext(context.Context) error {
	stub.calls.Add(1)
	return stub.err
}

func TestReadinessSucceedsWhenDatabasePingSucceeds(t *testing.T) {
	pinger := &pingerStub{}
	readiness := NewReadiness(pinger)

	require.NoError(t, readiness.CheckReadiness(context.Background()))
	require.Equal(t, int32(1), pinger.calls.Load())
}

func TestReadinessFailsWhenDatabasePingFails(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	readiness := NewReadiness(&pingerStub{err: databaseErr})

	require.ErrorIs(t, readiness.CheckReadiness(context.Background()), databaseErr)
}

func TestReadinessRejectsDrainingConcurrentlyWithDatabaseCheck(t *testing.T) {
	pinger := &blockingPinger{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	readiness := NewReadiness(pinger)
	result := make(chan error, 1)

	go func() {
		result <- readiness.CheckReadiness(context.Background())
	}()

	select {
	case <-pinger.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for database check")
	}

	readiness.MarkDraining()
	close(pinger.release)

	require.ErrorIs(t, <-result, errDraining)
}

func TestReadinessSkipsDatabaseCheckWhileDraining(t *testing.T) {
	pinger := &pingerStub{}
	readiness := NewReadiness(pinger)
	readiness.MarkDraining()

	require.ErrorIs(t, readiness.CheckReadiness(context.Background()), errDraining)
	require.Zero(t, pinger.calls.Load())
}

type blockingPinger struct {
	started chan struct{}
	release chan struct{}
}

func (pinger *blockingPinger) PingContext(context.Context) error {
	close(pinger.started)
	<-pinger.release
	return nil
}
