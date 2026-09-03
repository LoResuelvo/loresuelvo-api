package scheduler

import (
	"context"
	"time"
)

type taskMock struct {
	contexts []context.Context
	err      error
	invoked  chan struct{}
}

func (task *taskMock) UrgentNotification(ctx context.Context) error {
	task.contexts = append(task.contexts, ctx)
	if task.invoked != nil {
		task.invoked <- struct{}{}
	}
	return task.err
}

type tryLockerMock struct {
	acquired bool
	calls    int
	key      string
}

func (locker *tryLockerMock) TryWithinLock(_ context.Context, key string, operation func() error) (bool, error) {
	locker.calls++
	locker.key = key
	if !locker.acquired {
		return false, nil
	}
	return true, operation()
}

type calendarTaskMock struct {
	calls int
	err   error
}

func (task *calendarTaskMock) Sync(context.Context) error {
	task.calls++
	return task.err
}

type blockingUrgentTask struct {
	started chan struct{}
	release chan struct{}
}

func (task *blockingUrgentTask) UrgentNotification(context.Context) error {
	close(task.started)
	<-task.release
	return nil
}

type blockingCalendarTask struct {
	started chan struct{}
	release chan struct{}
}

func (task *blockingCalendarTask) Sync(context.Context) error {
	close(task.started)
	<-task.release
	return nil
}

type countingUrgentTask struct {
	called chan struct{}
}

func (task *countingUrgentTask) UrgentNotification(context.Context) error {
	task.called <- struct{}{}
	return nil
}

type countingCalendarTask struct {
	called chan struct{}
}

func (task *countingCalendarTask) Sync(context.Context) error {
	task.called <- struct{}{}
	return nil
}

type manualTicker struct {
	ticks   chan time.Time
	stopped chan struct{}
}

func (ticker *manualTicker) Chan() <-chan time.Time { return ticker.ticks }

func (ticker *manualTicker) Stop() { close(ticker.stopped) }
