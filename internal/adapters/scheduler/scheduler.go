package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	UrgentNotificationLockKey = "loresuelvo:scheduler:urgent-notifications"
	CalendarSyncLockKey       = "loresuelvo:scheduler:calendar-sync"
)

type TryLocker interface {
	TryWithinLock(ctx context.Context, resource string, operation func() error) (bool, error)
}

type Option func(*runConfig)

type runConfig struct {
	locker  TryLocker
	lockKey string
}

func WithLock(locker TryLocker, key string) Option {
	return func(config *runConfig) {
		config.locker = locker
		config.lockKey = key
	}
}

type Task interface {
	UrgentNotification(ctx context.Context) error
}

type ticker interface {
	Chan() <-chan time.Time
	Stop()
}

type timeTicker struct {
	*time.Ticker
}

func (t timeTicker) Chan() <-chan time.Time {
	return t.C
}

type Scheduler struct {
	interval  time.Duration
	task      Task
	newTicker func(time.Duration) ticker
	runConfig
}

func NewScheduler(interval time.Duration, task Task, options ...Option) *Scheduler {
	scheduler := &Scheduler{
		interval: interval,
		task:     task,
		newTicker: func(interval time.Duration) ticker {
			return timeTicker{Ticker: time.NewTicker(interval)}
		},
	}
	for _, option := range options {
		option(&scheduler.runConfig)
	}
	return scheduler
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := s.newTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			if err := s.RunOnce(ctx); err != nil {
				slog.Error("scheduler task failed", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	return s.run(ctx, s.task.UrgentNotification)
}

func (s *Scheduler) run(ctx context.Context, task func(context.Context) error) error {
	if s.locker == nil {
		return task(ctx)
	}
	acquired, err := s.locker.TryWithinLock(ctx, s.lockKey, func() error {
		return task(ctx)
	})
	if err != nil {
		return fmt.Errorf("coordinating scheduled task: %w", err)
	}
	if !acquired {
		return nil
	}
	return nil
}
