package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type CalendarSyncTask interface {
	Sync(context.Context) error
}

type CalendarSyncRunner struct {
	task      CalendarSyncTask
	newTicker func(time.Duration) ticker
	runConfig
}

func NewCalendarSyncRunner(task CalendarSyncTask, options ...Option) *CalendarSyncRunner {
	runner := &CalendarSyncRunner{
		task: task,
		newTicker: func(interval time.Duration) ticker {
			return timeTicker{Ticker: time.NewTicker(interval)}
		},
	}
	for _, option := range options {
		option(&runner.runConfig)
	}
	return runner
}

func (runner *CalendarSyncRunner) RunOnce(ctx context.Context) error {
	if runner.locker == nil {
		return runner.task.Sync(ctx)
	}
	acquired, err := runner.locker.TryWithinLock(ctx, runner.lockKey, func() error {
		return runner.task.Sync(ctx)
	})
	if err != nil {
		return fmt.Errorf("coordinating calendar sync: %w", err)
	}
	if !acquired {
		return nil
	}
	return nil
}

func (runner *CalendarSyncRunner) Run(ctx context.Context) {
	ticker := runner.newTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			if err := runner.RunOnce(ctx); err != nil {
				slog.Error("calendar sync scheduler task failed", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
