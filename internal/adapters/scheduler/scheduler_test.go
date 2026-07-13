package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskMock struct {
	contexts []context.Context
	err      error
	invoked  chan struct{}
}

func (m *taskMock) UrgentNotification(ctx context.Context) error {
	m.contexts = append(m.contexts, ctx)
	if m.invoked != nil {
		m.invoked <- struct{}{}
	}
	return m.err
}

type manualTicker struct {
	ticks   chan time.Time
	stopped chan struct{}
}

func (t *manualTicker) Chan() <-chan time.Time { return t.ticks }

func (t *manualTicker) Stop() { close(t.stopped) }

func TestSchedulerRunOnceInvokesTaskWithContext(t *testing.T) {
	task := &taskMock{invoked: make(chan struct{}, 1)}
	scheduler := NewScheduler(time.Hour, task)

	err := scheduler.RunOnce(t.Context())

	require.NoError(t, err)
	require.Len(t, task.contexts, 1)
	assert.Same(t, t.Context(), task.contexts[0])
}

func TestSchedulerRunOncePropagatesTaskError(t *testing.T) {
	expectedErr := errors.New("task failed")
	scheduler := NewScheduler(time.Hour, &taskMock{err: expectedErr})

	err := scheduler.RunOnce(t.Context())

	assert.ErrorIs(t, err, expectedErr)
}

func TestSchedulerRunExecutesOnTickAndStopsWhenContextIsCancelled(t *testing.T) {
	task := &taskMock{invoked: make(chan struct{}, 1)}
	manual := &manualTicker{ticks: make(chan time.Time), stopped: make(chan struct{})}
	scheduler := NewScheduler(time.Hour, task)
	scheduler.newTicker = func(time.Duration) ticker { return manual }
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	manual.ticks <- time.Now()
	select {
	case <-task.invoked:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not execute its task after a tick")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}
	select {
	case <-manual.stopped:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop its ticker")
	}
}
